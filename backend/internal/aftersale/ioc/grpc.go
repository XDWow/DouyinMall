package ioc

import (
	"fmt"
	"net"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	aftersalegrpc "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/grpc"
	aftersalev1service "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1/aftersaleservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitexserver "github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
)

func InitGRPCServer(cfg aftersaleconfig.Config, handler *aftersalegrpc.Handler) (kitexserver.Server, error) {
	port := cfg.GRPC.Server.Port
	if port == 0 {
		port = 8097
	}
	serviceName := cfg.GRPC.Server.Name
	if serviceName == "" {
		serviceName = "aftersale.service"
	}

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("resolve grpc addr failed: %w", err)
	}

	options := []kitexserver.Option{
		kitexserver.WithServiceAddr(addr),
		kitexserver.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	}

	if len(cfg.Etcd.Endpoints) > 0 {
		registry, err := etcd.NewEtcdRegistry(cfg.Etcd.Endpoints)
		if err != nil {
			return nil, fmt.Errorf("init etcd registry failed: %w", err)
		}
		options = append(options, kitexserver.WithRegistry(registry))
	}

	return aftersalev1service.NewServer(handler, options...), nil
}
