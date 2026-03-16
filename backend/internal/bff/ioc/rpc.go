package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1/agentservice"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamclient"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitAgentClient 初始化 Agent gRPC 客户端。
// 若配置了 rpc.agent.direct_addr，则直连该地址（适用于宿主机调试）；
// 否则走 etcd 服务发现（适用于 Docker 部署）。
func InitAgentClient() agentservice.Client {
	if addr := viper.GetString("rpc.agent.direct_addr"); addr != "" {
		c, err := agentservice.NewClient(agentServiceName(), client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("创建 Agent RPC 客户端失败(direct): %w", err))
		}
		return c
	}

	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}
	c, err := agentservice.NewClient(agentServiceName(), client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("创建 Agent RPC 客户端失败: %w", err))
	}
	return c
}

// InitAgentStreamClient 初始化 Agent gRPC 流式客户端
func InitAgentStreamClient() agentservice.StreamClient {
	if addr := viper.GetString("rpc.agent.direct_addr"); addr != "" {
		c, err := agentservice.NewStreamClient(agentServiceName(),
			streamclient.WithHostPorts(addr),
		)
		if err != nil {
			panic(fmt.Errorf("创建 Agent Stream 客户端失败(direct): %w", err))
		}
		return c
	}

	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}
	c, err := agentservice.NewStreamClient(agentServiceName(),
		streamclient.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("创建 Agent Stream 客户端失败: %w", err))
	}
	return c
}

func agentServiceName() string {
	name := viper.GetString("rpc.agent.service_name")
	if name == "" {
		return "agent-service"
	}
	return name
}

func etcdEndpoints() []string {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	if len(endpoints) == 0 {
		endpoints = []string{"localhost:2379"}
	}
	return endpoints
}
