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
		panic(fmt.Errorf("鍒涘缓 etcd 鏈嶅姟鍙戠幇澶辫触: %w", err))
	}
	c, err := paymentservice.NewClient("payment.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 payment 瀹㈡埛绔け璐? %w", err))
	}
	return c
}


