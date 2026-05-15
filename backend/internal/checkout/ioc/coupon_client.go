package ioc

import (
	"fmt"

	couponservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1/couponservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitCouponClient() couponservice.Client {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver for coupon client: %w", err))
	}
	c, err := couponservice.NewClient("coupon.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create coupon client: %w", err))
	}
	return c
}
