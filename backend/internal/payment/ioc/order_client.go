package ioc

import (
	"fmt"

	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitOrderClient 初始化订单服务客户端
func InitOrderClient() orderv1.Client {
	// 初始化 etcd 服务发现
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}

	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd 服务发现失败: %w", err))
	}

	// 创建订单服务客户端
	orderClient, err := orderv1.NewClient(
		"order.service", // 服务名
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("创建订单服务客户端失败: %w", err))
	}

	return orderClient
}
