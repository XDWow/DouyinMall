package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/transport/grpc"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1/couponservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(couponHandler *grpc.CouponHandler) server.Server {
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
			Name: "coupon.service",
			Port: 8095,
		},
	}
	viper.UnmarshalKey("grpc", &grpcCfg)
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", grpcCfg.Server.Port))

	return couponv1.NewServer(
		couponHandler,
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: grpcCfg.Server.Name,
		}),
	)
}
