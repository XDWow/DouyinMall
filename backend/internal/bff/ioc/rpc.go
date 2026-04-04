package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1/agentservice"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamclient"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitAgentClient 鍒濆鍖?Agent gRPC 瀹㈡埛绔€?
// 鑻ラ厤缃簡 rpc.agent.direct_addr锛屽垯鐩磋繛璇ュ湴鍧€锛堥€傜敤浜庡涓绘満璋冭瘯锛夛紱
// 鍚﹀垯璧?etcd 鏈嶅姟鍙戠幇锛堥€傜敤浜?Docker 閮ㄧ讲锛夈€?
func InitAgentClient() agentservice.Client {
	if addr := viper.GetString("rpc.agent.direct_addr"); addr != "" {
		c, err := agentservice.NewClient(agentServiceName(), client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("鍒涘缓 Agent RPC 瀹㈡埛绔け璐?direct): %w", err))
		}
		return c
	}

	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd resolver 澶辫触: %w", err))
	}
	c, err := agentservice.NewClient(agentServiceName(), client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 Agent RPC 瀹㈡埛绔け璐? %w", err))
	}
	return c
}

// InitAgentStreamClient 鍒濆鍖?Agent gRPC 娴佸紡瀹㈡埛绔?
func InitAgentStreamClient() agentservice.StreamClient {
	if addr := viper.GetString("rpc.agent.direct_addr"); addr != "" {
		c, err := agentservice.NewStreamClient(agentServiceName(),
			streamclient.WithHostPorts(addr),
		)
		if err != nil {
			panic(fmt.Errorf("鍒涘缓 Agent Stream 瀹㈡埛绔け璐?direct): %w", err))
		}
		return c
	}

	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd resolver 澶辫触: %w", err))
	}
	c, err := agentservice.NewStreamClient(agentServiceName(),
		streamclient.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 Agent Stream 瀹㈡埛绔け璐? %w", err))
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


