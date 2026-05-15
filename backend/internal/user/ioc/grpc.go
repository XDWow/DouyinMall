package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/user/handler"
	userv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1/userservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(userHandler *handler.UserServiceServer) server.Server {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		panic("etcd.endpoints is empty")
	}

	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 注册中心失败: %w", err))
	}

	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Errorf("解析服务地址失败: %w", err))
	}

	svr := userv1.NewServer(
		userHandler,
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	)

	return svr
}
