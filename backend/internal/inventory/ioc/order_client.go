package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitOrderClient() orderservice.Client {
	etcdCfg := config.EtcdConfig{}
	viper.UnmarshalKey("etcd", &etcdCfg)
	if len(etcdCfg.Endpoints) == 0 {
		panic("etcd endpoints are empty")
	}

	r, err := etcd.NewEtcdResolver(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver: %w", err))
	}

	cli, err := orderservice.NewClient(
		"order.service",
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("create order client: %w", err))
	}

	return cli
}
