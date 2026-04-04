//go:build legacy_agent

package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/handler"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1/agentservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(agentHandler *handler.AgentHandler) server.Server {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("閸掓稑缂?etcd 濞夈劌鍞芥稉顓炵妇婢惰精瑙? %w", err))
	}

	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))

	svr := agentv1.NewServer(
		agentHandler,
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	)
	return svr
}


