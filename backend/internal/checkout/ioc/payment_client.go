package ioc

import (
	"fmt"

	paymentservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitPaymentClient() paymentservice.Client {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver for payment client: %w", err))
	}
	c, err := paymentservice.NewClient("payment.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create payment client: %w", err))
	}
	return c
}
