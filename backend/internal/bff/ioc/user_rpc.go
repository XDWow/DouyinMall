package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1/userservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitUserClient() userservice.Client {
	name := viper.GetString("rpc.user.service_name")
	if name == "" {
		name = "user-service"
	}

	if addr := viper.GetString("rpc.user.direct_addr"); addr != "" {
		c, err := userservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("创建 User RPC 客户端失败(direct): %w", err))
		}
		return c
	}

	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}
	c, err := userservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("创建 User RPC 客户端失败: %w", err))
	}
	return c
}
