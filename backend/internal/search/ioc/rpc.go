package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// 鍒濆鍖?Product Service RPC 瀹㈡埛绔紙鐢ㄤ簬鎵归噺鍚屾鏁版嵁锛?
func InitProductClient() productservice.Client {
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

	// 鍒涘缓 Product Service 瀹㈡埛绔?
	productClient, err := productservice.NewClient(
		"product-service", // 鏈嶅姟鍚嶇О锛岄渶瑕佷笌 Product Service 娉ㄥ唽鐨勫悕绉颁竴鑷?
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 Product Service RPC 瀹㈡埛绔け璐? %w", err))
	}

	return productClient
}


