package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1/searchservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

// ==================== etcd helper ====================

func etcdEndpoints() []string {
	endpoints := viper.GetStringSlice("etcd.endpoints")
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	if len(endpoints) == 0 {
		endpoints = []string{"localhost:2379"}
	}
	return endpoints
}

func serviceName(key, fallback string) string {
	name := viper.GetString(key)
	if name == "" {
		return fallback
	}
	return name
}

// ==================== 下游 Kitex 客户端 ====================

func InitSearchClient() searchservice.Client {
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("etcd resolver 创建失败: %w", err))
	}
	c, err := searchservice.NewClient(
		serviceName("rpc.search.service_name", "search.service"),
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("search 客户端创建失败: %w", err))
	}
	return c
}

func InitProductClient() productservice.Client {
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("etcd resolver 创建失败: %w", err))
	}
	c, err := productservice.NewClient(
		serviceName("rpc.product.service_name", "product.service"),
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("product 客户端创建失败: %w", err))
	}
	return c
}

func InitCartClient() cartservice.Client {
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("etcd resolver 创建失败: %w", err))
	}
	c, err := cartservice.NewClient(
		serviceName("rpc.cart.service_name", "cart.service"),
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("cart 客户端创建失败: %w", err))
	}
	return c
}

func InitCheckoutClient() checkoutservice.Client {
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("etcd resolver 创建失败: %w", err))
	}
	c, err := checkoutservice.NewClient(
		serviceName("rpc.checkout.service_name", "checkout.service"),
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("checkout 客户端创建失败: %w", err))
	}
	return c
}

func InitOrderClient() orderservice.Client {
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("etcd resolver 创建失败: %w", err))
	}
	c, err := orderservice.NewClient(
		serviceName("rpc.order.service_name", "order.service"),
		client.WithResolver(r),
	)
	if err != nil {
		panic(fmt.Errorf("order 客户端创建失败: %w", err))
	}
	return c
}
