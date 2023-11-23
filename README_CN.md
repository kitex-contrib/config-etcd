# config-etcd (*这是一个社区驱动的项目*)

[English](./README.md)

使用 **etcd** 作为 **Kitex** 的服务治理配置中心

## 用法

### 基本使用

#### 服务端

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/kitex-examples/kitex_gen/api"
	"github.com/cloudwego/kitex-examples/kitex_gen/api/echo"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/kitex-contrib/config-etcd/etcd"
	etcdServer "github.com/kitex-contrib/config-etcd/server"
)

var _ api.Echo = &EchoImpl{}

// EchoImpl implements the last service interface defined in the IDL.
type EchoImpl struct{}

// Echo implements the Echo interface.
func (s *EchoImpl) Echo(ctx context.Context, req *api.Request) (resp *api.Response, err error) {
	klog.Info("echo called")
	return &api.Response{Message: req.Message}, nil
}

func main() {
	serviceName := "ServiceName"// your server-side service name
	etcdClient, err := etcd.NewClient(etcd.Options{})
	if err!=nil {
		panic(err)
	}
	svr := echo.NewServer(
		new(EchoImpl),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
		server.WithSuite(etcdServer.NewSuite(serviceName, etcdClient)),
	)
	if err := svr.Run(); err != nil {
		log.Println("server stopped with error:", err)
	} else {
		log.Println("server stopped")
	}
}

```

#### Client

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/kitex-examples/kitex_gen/api"
	"github.com/cloudwego/kitex-examples/kitex_gen/api/echo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	etcdclient "github.com/kitex-contrib/config-etcd/client"
	"github.com/kitex-contrib/config-etcd/etcd"
	"github.com/kitex-contrib/config-etcd/utils"
)

type configLog struct{}

func (cl *configLog) Apply(opt *utils.Options) {
	fn := func(k *etcd.Key) {
		klog.Infof("etcd config %v", k)
	}
	opt.EtcdCustomFunctions = append(opt.EtcdCustomFunctions, fn)
}

func main() {
	etcdClient, err := etcd.NewClient(etcd.Options{})
	if err != nil {
		panic(err)
	}

	cl := &configLog{}

	serviceName := "ServiceName"// your server-side service name
	clientName := "ClientName"// your client-side service name
	client, err := echo.NewClient(
		serviceName,
		client.WithHostPorts("0.0.0.0:8888"),
		client.WithSuite(etcdclient.NewSuite(serviceName, clientName, etcdClient, cl)),
	)
	if err != nil {
		log.Fatal(err)
	}
	for {
		req := &api.Request{Message: "my request"}
		resp, err := client.Echo(context.Background(), req)
		if err != nil {
			klog.Errorf("take request error: %v", err)
		} else {
			klog.Infof("receive response %v", resp)
		}
		time.Sleep(time.Second * 10)
	}
}
```

### Etcd 配置

#### CustomFunction

允许用户自定义 etcd 的参数来自定义参数 `Key`.
```go
type Key struct {
    Prefix string
    Path   string
}
```

#### Options 默认值

| 参数               | 变量默认值                                                       | 作用                                                                                                                     |
|------------------|-------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------|
| Address          | 127.0.0.1                                                   | Etcd 服务器地址                                                                                                             |
| Port             | 2379                                                        | Etcd 服务器端口                                                                                                             |
| Prefix           | /KitexConfig                                                | Etcd 中的 prefix                                                                                                         |
| ClientPathFormat | {{.ClientServiceName}}/{{.ServerServiceName}}/{{.Category}} | 使用 go [template](https://pkg.go.dev/text/template) 语法渲染生成对应的 ID, 使用 `ClientServiceName` `ServiceName` `Category` 三个元数据 |
| ServerPathFormat | {{.ServerServiceName}}/{{.Category}}                        | 使用 go [template](https://pkg.go.dev/text/template) 语法渲染生成对应的 ID, 使用 `ServiceName` `Category` 两个元数据                     |

#### 治理策略
下面例子中的 configPath 以及 configPrefix 均使用默认值，服务名称为 ServiceName，客户端名称为 ClientName

##### 限流 Category=limit
> 限流目前只支持服务端，所以 ClientServiceName 为空。

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/limiter/item_limiter.go#L33)

| 字段               | 说明               |
|------------------|------------------|
| connection_limit | 最大并发数量           | 
| qps_limit        | 每 100ms 内的最大请求数量 | 

例子：

> configPath: /KitexConfig/ServiceName/limit

```json
{
  "connection_limit": 100, 
  "qps_limit": 2000        
}
```
注：

- 限流配置的粒度是 Server 全局，不分 client、method
- 「未配置」或「取值为 0」表示不开启
- connection_limit 和 qps_limit 可以独立配置，例如 connection_limit = 100, qps_limit = 0

##### 重试 Category=retry

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/retry/policy.go#L63)

| 参数                            | 说明                                 |
|-------------------------------|------------------------------------|
| type                          | 0: failure_policy 1: backup_policy | 
| failure_policy.backoff_policy | 可以设置的策略： `fixed` `none` `random`   | 

例子：

> configPath: /KitexConfig/ClientName/ServiceName/retry

```json
{
    "*": {  
        "enable": true,
        "type": 0,                 
        "failure_policy": {
            "stop_policy": {
                "max_retry_times": 3,
                "max_duration_ms": 2000,
                "cb_policy": {
                    "error_rate": 0.3
                }
            },
            "backoff_policy": {
                "backoff_type": "fixed", 
                "cfg_items": {
                    "fix_ms": 50
                }
            },
            "retry_same_node": false
        }
    },
    "echo": { 
        "enable": true,
        "type": 1,                 
        "backup_policy": {
            "retry_delay_ms": 100,
            "retry_same_node": false,
            "stop_policy": {
                "max_retry_times": 2,
                "max_duration_ms": 300,
                "cb_policy": {
                    "error_rate": 0.2
                }
            }
        }
    }
}
```
注：retry.Container 内置支持用 * 通配符指定默认配置（详见 [getRetryer](https://github.com/cloudwego/kitex/blob/v0.5.1/pkg/retry/retryer.go#L240) 方法）

##### 超时 Category=rpc_timeout

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/rpctimeout/item_rpc_timeout.go#L42)

例子：

> configPath: /KitexConfig/ClientName/ServiceName/rpc_timeout

```json
{
  "*": {
    "conn_timeout_ms": 100, 
    "rpc_timeout_ms": 3000
  },
  "echo": {
    "conn_timeout_ms": 50,
    "rpc_timeout_ms": 1000
  }
}
```
注：kitex 的熔断实现目前不支持修改全局默认配置（详见 [initServiceCB](https://github.com/cloudwego/kitex/blob/v0.5.1/pkg/circuitbreak/cbsuite.go#L195)）

##### 熔断: Category=circuit_break

[JSON Schema](https://github.com/cloudwego/kitex/blob/develop/pkg/circuitbreak/item_circuit_breaker.go#L30)

| 参数         | 说明       |
|------------|----------|
| min_sample | 最小的统计样本数 | 

例子：

echo 方法使用下面的配置（0.3、100），其他方法使用全局默认配置（0.5、200）

> configPath: /KitexConfig/ClientName/ServiceName/circuit_break

```json
{
  "echo": {
    "enable": true,
    "err_rate": 0.3,
    "min_sample": 100
  }
}
```
### 更多信息

更多示例请参考 [example](https://github.com/kitex-contrib/config-etcd/tree/main/example)

## Compatibility
因为 grpc 兼容的问题，Go 的版本必须 >= 1.19

主要贡献者： [ViolaPioggia](https://github.com/violapioggia)

