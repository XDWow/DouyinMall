package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// 鍒濆鍖栬鍗曟湇鍔″鎴风锛堢敤浜庡悓姝ヨ皟鐢ㄦ洿鏂拌鍗曠姸鎬侊級
func InitOrderClient() orderservice.Client {
	// etcd閰嶇疆
	etcdCfg := config.EtcdConfig{
		Endpoints: []string{"localhost:12379"},
	}
	viper.UnmarshalKey("etcd", &etcdCfg)

	r, err := etcd.NewEtcdResolver(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd resolver 澶辫触: %w", err))
	}

	// 鍒涘缓璁㈠崟鏈嶅姟瀹㈡埛绔?
	cli, err := orderservice.NewClient(
		"order.service",
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓璁㈠崟鏈嶅姟瀹㈡埛绔け璐? %w", err))
	}

	return cli
}


