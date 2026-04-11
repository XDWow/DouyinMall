package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/order/transport/grpc"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(orderHandler *grpc.OrderHandler) server.Server {
	// 初始化 etcd 注册中心
	endpoints := viper.GetStringSlice("etcd.endpoints")
	// 若配置为单个字符串，则转为切片
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 注册中心失败: %w", err))
	}

	// 服务监听配置
	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))

	svr := orderv1.NewServer(
		orderHandler,
		server.WithRegistry(r),       // 注册到 etcd
		server.WithServiceAddr(addr), // 监听地址
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	)

	return svr
}


