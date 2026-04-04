package ioc

import (
	"fmt"

	productservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitProductClient() productservice.Client {
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
	c, err := productservice.NewClient("product.service", client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 product 瀹㈡埛绔け璐? %w", err))
	}
	return c
}


