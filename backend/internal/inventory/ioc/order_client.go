package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitOrderClient 初始化订单服务客户端（用于同步调用更新订单状态等）
func InitOrderClient() orderservice.Client {
	// etcd 配置
	etcdCfg := config.EtcdConfig{
		Endpoints: []string{"localhost:12379"},
	}
	viper.UnmarshalKey("etcd", &etcdCfg)

	r, err := etcd.NewEtcdResolver(etcdCfg.Endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}

	// 创建订单服务客户端
	cli, err := orderservice.NewClient(
		"order.service",
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("创建订单服务客户端失败: %w", err))
	}

	return cli
}


