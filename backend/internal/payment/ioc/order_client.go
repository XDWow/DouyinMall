package ioc

import (
	"fmt"

	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitOrderClient 鍒濆鍖栬鍗曟湇鍔″鎴风
func InitOrderClient() orderv1.Client {
	// 鍒濆鍖?etcd 鏈嶅姟鍙戠幇
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

	// 鍒涘缓璁㈠崟鏈嶅姟瀹㈡埛绔?
	orderClient, err := orderv1.NewClient(
		"order.service", // 鏈嶅姟鍚?
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓璁㈠崟鏈嶅姟瀹㈡埛绔け璐? %w", err))
	}

	return orderClient
}


