package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// 初始化 Product Service RPC 客户端（用于批量同步数据）
func InitProductClient() productservice.Client {
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

	// 创建 Product Service 客户端
	productClient, err := productservice.NewClient(
		"product-service", // 服务名称，需要与 Product Service 注册的名称一致
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("创建 Product Service RPC 客户端失败: %w", err))
	}

	return productClient
}
