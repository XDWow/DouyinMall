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
	// 鍒濆鍖?etcd 娉ㄥ唽涓績
	etcdCfg := config.EtcdConfig{
		Endpoints: []string{"localhost:12379"},
	}
	viper.UnmarshalKey("etcd", &etcdCfg)
	r, err := etcd.NewEtcdRegistry(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd 娉ㄥ唽涓績澶辫触: %w", err))
	}

	// 鏈嶅姟閰嶇疆
	grpcCfg := config.GRPCConfig{
		Server: config.ServerConfig{
			Name: "coupon.service",
			Port: 8095,
		},
	}
	viper.UnmarshalKey("grpc", &grpcCfg)
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", grpcCfg.Server.Port))

	// 鍒涘缓 Kitex 鏈嶅姟
	svr := couponv1.NewServer(
		couponHandler,
		server.WithRegistry(r),       // 娉ㄥ唽鍒?etcd
		server.WithServiceAddr(addr), // 鏈嶅姟鍦板潃
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: grpcCfg.Server.Name,
		}),
	)

	return svr
}


