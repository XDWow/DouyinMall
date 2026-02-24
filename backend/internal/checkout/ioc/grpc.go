package ioc

import (
	"fmt"
	"net"

	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/checkout/transport/grpc"
	checkoutservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(handler *transportgrpc.CheckoutHandler) server.Server {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 注册中心失败: %w", err))
	}

	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))

	svr := checkoutservice.NewServer(
		handler,
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	)
	return svr
}
