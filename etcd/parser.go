package etcd

import (
	"encoding/json"
)

const (
	EtcdDefaultServerAddr   = "127.0.0.1"
	EtcdDefaultPort         = 2379
	EtcdDefaultConfigPrefix = "KitexConfig"
	EtcdDefaultClientPath   = "{{.ClientServiceName}}/{{.ServerServiceName}}/{{.Category}}"
	EtcdDefaultServerPath   = "{{.ServerServiceName}}/{{.Category}}"
)

var _ ConfigParser = &parser{}

// CustomFunction use for customize the config parameters.
type CustomFunction func(*Key)

// ConfigParamConfig use for render the path or prefix info by go template, ref: https://pkg.go.dev/text/template
// The fixed key shows as below.
type ConfigParamConfig struct {
	Category          string
	ClientServiceName string
	ServerServiceName string
}

// ConfigParser the parser for etcd config.
type ConfigParser interface {
	Decode(data string, config interface{}) error
}

type parser struct{}

// Decode decodes the data to struct in specified format.
func (p *parser) Decode(data string, config interface{}) error {
	return json.Unmarshal([]byte(data), config)
}

// DefaultConfigParse default etcd config parser.
func defaultConfigParse() ConfigParser {
	return &parser{}
}
