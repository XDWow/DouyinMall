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

	resolver, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("create etcd resolver for payment failed: %w", err))
	}

	client, err := paymentservice.NewClient("payment.service", client.WithResolver(resolver))
	if err != nil {
		panic(fmt.Errorf("create payment client failed: %w", err))
	}
	return client
}


