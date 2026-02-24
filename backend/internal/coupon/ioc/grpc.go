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
			Name: "coupon.service",
			Port: 8095,
		},
	}
	viper.UnmarshalKey("grpc", &grpcCfg)
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", grpcCfg.Server.Port))

	// 创建 Kitex 服务
	svr := couponv1.NewServer(
		couponHandler,
		server.WithRegistry(r),       // 注册到 etcd
		server.WithServiceAddr(addr), // 服务地址
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: grpcCfg.Server.Name,
		}),
	)

	return svr
}
