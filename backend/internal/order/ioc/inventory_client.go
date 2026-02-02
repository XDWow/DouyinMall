package ioc

import (
	"fmt"

	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// InitInventoryClient 初始化库存服务RPC客户端
func InitInventoryClient() inventoryv1.Client {
	// 从配置读取 etcd 地址
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}

	// 创建 etcd resolver
	r, err := etcd.NewEtcdResolver(endpoints)
	if err != nil {
		panic(fmt.Errorf("创建 etcd resolver 失败: %w", err))
	}

	// 服务发现名称（需要和库存服务注册时的名称一致）
	serviceName := "inventory-service"

	// 创建 Kitex 客户端
	c, err := inventoryv1.NewClient(
		serviceName,
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("创建库存服务客户端失败: %w", err))
	}

	return c
}
