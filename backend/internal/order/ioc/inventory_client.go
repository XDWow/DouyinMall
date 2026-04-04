package ioc

import (
	"fmt"

	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitInventoryClient 鍒濆鍖栧簱瀛樻湇鍔PC瀹㈡埛绔?
func InitInventoryClient() inventoryv1.Client {
	// 浠庨厤缃鍙?etcd 鍦板潃
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}

	// 鍒涘缓 etcd resolver
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd resolver 澶辫触: %w", err))
	}

	// 鏈嶅姟鍙戠幇鍚嶇О锛堥渶瑕佸拰搴撳瓨鏈嶅姟娉ㄥ唽鏃剁殑鍚嶇О涓€鑷达級
	serviceName := "inventory-service"

	// 鍒涘缓 Kitex 瀹㈡埛绔?
	c, err := inventoryv1.NewClient(
		serviceName,
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓搴撳瓨鏈嶅姟瀹㈡埛绔け璐? %w", err))
	}

	return c
}


