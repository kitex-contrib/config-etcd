// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcd

import (
	"bytes"
	"context"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"strconv"
	"sync"
	"text/template"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/cloudwego/kitex/pkg/klog"
	"go.uber.org/zap"
)

var (
	m      sync.Mutex
	ctxMap = make(map[string]context.CancelFunc)
)

type Key struct {
	Prefix string
	Path   string
}

type Client interface {
	SetParser(ConfigParser)
	ClientConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (Key, error)
	ServerConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (Key, error)
	RegisterConfigCallback(ctx context.Context, key string, clientId int64, callback func(string, ConfigParser))
	DeregisterConfig(key string, uniqueId int64)
}

type client struct {
	ecli *clientv3.Client
	// support customise parser
	parser             ConfigParser
	etcdDialTimeout    time.Duration
	prefixTemplate     *template.Template
	serverPathTemplate *template.Template
	clientPathTemplate *template.Template
}

// Options etcd config options. All the fields have default value.
type Options struct {
	Node             []string
	Prefix           string
	ServerPathFormat string
	clientPathFormat string
	DialTimeout      time.Duration
	LoggerConfig     *zap.Config
	ConfigParser     ConfigParser
}

// New Create a default etcd client
// It can create a client with default config by env variable.
// See: env.go
func New(opts Options) (Client, error) {
	if opts.Node == nil {
		opts.Node = []string{EtcdDefaultNode}
	}
	if opts.ConfigParser == nil {
		opts.ConfigParser = defaultConfigParse()
	}
	if opts.Prefix == "" {
		opts.Prefix = EtcdDefaultConfigPrefix
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = EtcdDefaultDialTimeOut
	}
	if opts.ServerPathFormat == "" {
		opts.ServerPathFormat = EtcdDefaultServerPath
	}
	if opts.clientPathFormat == "" {
		opts.clientPathFormat = EtcdDefaultClientPath
	}
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints: opts.Node,
		LogConfig: opts.LoggerConfig,
	})
	if err != nil {
		return nil, err
	}
	prefixTemplate, err := template.New("prefix").Parse(opts.Prefix)
	if err != nil {
		return nil, err
	}
	serverNameTemplate, err := template.New("serverName").Parse(opts.ServerPathFormat)
	if err != nil {
		return nil, err
	}
	clientNameTemplate, err := template.New("clientName").Parse(opts.clientPathFormat)
	if err != nil {
		return nil, err
	}
	c := &client{
		ecli:               etcdClient,
		parser:             opts.ConfigParser,
		etcdDialTimeout:    opts.DialTimeout,
		prefixTemplate:     prefixTemplate,
		serverPathTemplate: serverNameTemplate,
		clientPathTemplate: clientNameTemplate,
	}
	return c, nil
}

func (c *client) SetParser(parser ConfigParser) {
	c.parser = parser
}

func (c *client) ClientConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (Key, error) {
	return c.configParam(cpc, c.clientPathTemplate, cfs...)
}

func (c *client) ServerConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (Key, error) {
	return c.configParam(cpc, c.serverPathTemplate, cfs...)
}

// configParam render config parameters. All the parameters can be customized with CustomFunction.
// ConfigParam explain:
//  1. Prefix: KitexConfig by default.
//  2. ServerPath: {{.ServerServiceName}}/{{.Category}} by default.
//     ClientPath: {{.ClientServiceName}}/{{.ServerServiceName}}/{{.Category}} by default.
func (c *client) configParam(cpc *ConfigParamConfig, t *template.Template, cfs ...CustomFunction) (Key, error) {
	param := Key{}

	var err error
	param.Path, err = c.render(cpc, t)
	if err != nil {
		return param, err
	}
	param.Prefix, err = c.render(cpc, c.prefixTemplate)
	if err != nil {
		return param, err
	}

	for _, cf := range cfs {
		cf(&param)
	}
	return param, nil
}

func (c *client) render(cpc *ConfigParamConfig, t *template.Template) (string, error) {
	var tpl bytes.Buffer
	err := t.Execute(&tpl, cpc)
	if err != nil {
		return "", err
	}
	return tpl.String(), nil
}

// RegisterConfigCallback register the callback function to etcd client.
func (c *client) RegisterConfigCallback(ctx context.Context, key string, uniqueID int64, callback func(string, ConfigParser)) {
	clientCtx, cancel := context.WithCancel(context.Background())
	go func() {
		m.Lock()
		clientKey := key + "/" + strconv.FormatInt(uniqueID, 10)
		ctxMap[clientKey] = cancel
		m.Unlock()
		watchChan := c.ecli.Watch(ctx, key)
		for {
			select {
			case <-clientCtx.Done():
				return
			case watchResp := <-watchChan:
				for _, event := range watchResp.Events {
					eventType := event.Type
					// check the event type
					if eventType == mvccpb.PUT {
						// config is updated
						value := string(event.Kv.Value)
						klog.Debugf("[etcd] config key: %s updated,value is %s", key, value)
						callback(value, c.parser)
					} else if eventType == mvccpb.DELETE {
						// config is deleted
						klog.Debugf("[etcd] config key: %s deleted", key)
						callback("", c.parser)
					}
				}
			}
		}
	}()
	ctx, cancel = context.WithTimeout(context.Background(), c.etcdDialTimeout)
	defer cancel()
	data, err := c.ecli.Get(ctx, key)
	// the etcd client has handled the not exist error.
	if err != nil {
		klog.Debugf("[etcd] key: %s config get value failed", key)
		return
	}
	if data.Kvs == nil {
		callback("", c.parser)
		return
	}
	callback(string(data.Kvs[0].Value), c.parser)
}

func (c *client) DeregisterConfig(key string, uniqueID int64) {
	m.Lock()
	clientKey := key + "/" + strconv.FormatInt(uniqueID, 10)
	cancel := ctxMap[clientKey]
	cancel()
	m.Unlock()
}
