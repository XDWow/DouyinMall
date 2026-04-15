package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitOrderClient() orderservice.Client {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}
	c, err := orderservice.NewClient("order.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("创建订单服务客户端失败: %w", err))
	}
	return c
}


