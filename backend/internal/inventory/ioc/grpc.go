package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/transport/grpc"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(inventoryHandler *grpc.InventoryHandler) server.Server {
	// 初始化 etcd 注册中心
	etcdCfg := config.EtcdConfig{
		Endpoints: []string{"localhost:12379"},
	}
	viper.UnmarshalKey("etcd", &etcdCfg)
	r, err := etcd.NewEtcdRegistry(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 注册中心失败: %w", err))
	}

	// 服务配置
	grpcCfg := config.GRPCConfig{
		Server: config.ServerConfig{
			Name: "inventory.service",
			Port: 8094,
		},
	}
	viper.UnmarshalKey("grpc", &grpcCfg)
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", grpcCfg.Server.Port))

	// 创建 Kitex 服务
	svr := inventoryv1.NewServer(
		inventoryHandler,
		server.WithRegistry(r),       // 注册到 etcd
		server.WithServiceAddr(addr), // 服务监听地址
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: grpcCfg.Server.Name,
		}),
	)

	return svr
}


