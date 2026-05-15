package ioc

import (
	"fmt"

	inventoryservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitInventoryClient() inventoryservice.Client {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver for inventory client: %w", err))
	}
	c, err := inventoryservice.NewClient("inventory.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create inventory client: %w", err))
	}
	return c
}
