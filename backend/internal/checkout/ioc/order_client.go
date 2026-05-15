package ioc

import (
	"fmt"

	orderservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitOrderClient() orderservice.Client {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver for order client: %w", err))
	}
	c, err := orderservice.NewClient("order.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create order client: %w", err))
	}
	return c
}
