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
	etcdCfg := config.EtcdConfig{}
	viper.UnmarshalKey("etcd", &etcdCfg)
	if len(etcdCfg.Endpoints) == 0 {
		panic("etcd endpoints are empty")
	}

	r, err := etcd.NewEtcdRegistry(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd registry: %w", err))
	}

	grpcCfg := config.GRPCConfig{
		Server: config.ServerConfig{
			Name: "inventory.service",
			Port: 8094,
		},
	}
	viper.UnmarshalKey("grpc", &grpcCfg)
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", grpcCfg.Server.Port))

	return inventoryv1.NewServer(
		inventoryHandler,
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: grpcCfg.Server.Name,
		}),
	)
}
