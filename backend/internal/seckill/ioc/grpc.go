package ioc

import (
	"fmt"
	"net"

	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/seckill/transport/grpc"
	seckillservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(handler *transportgrpc.Handler) server.Server {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 注册中心失败: %w", err))
	}
	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	return seckillservice.NewServer(
		handler,
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: serviceName}),
	)
}


