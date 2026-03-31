package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	inventoryservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	productservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitCheckoutClient() checkoutservice.Client {
	name := serviceNameOrDefault("rpc.checkout.service_name", "checkout-service")
	if addr := viper.GetString("rpc.checkout.direct_addr"); addr != "" {
		c, err := checkoutservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("create checkout rpc client failed (direct): %w", err))
		}
		return c
	}
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("create etcd resolver failed: %w", err))
	}
	c, err := checkoutservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create checkout rpc client failed: %w", err))
	}
	return c
}

func InitSeckillClient() seckillservice.Client {
	name := serviceNameOrDefault("rpc.seckill.service_name", "seckill-service")
	if addr := viper.GetString("rpc.seckill.direct_addr"); addr != "" {
		c, err := seckillservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("create seckill rpc client failed (direct): %w", err))
		}
		return c
	}
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("create etcd resolver failed: %w", err))
	}
	c, err := seckillservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create seckill rpc client failed: %w", err))
	}
	return c
}

func InitOrderClient() orderservice.Client {
	name := serviceNameOrDefault("rpc.order.service_name", "order-service")
	if addr := viper.GetString("rpc.order.direct_addr"); addr != "" {
		c, err := orderservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("create order rpc client failed (direct): %w", err))
		}
		return c
	}
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("create etcd resolver failed: %w", err))
	}
	c, err := orderservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create order rpc client failed: %w", err))
	}
	return c
}

func InitProductClient() productservice.Client {
	name := serviceNameOrDefault("rpc.product.service_name", "product-service")
	if addr := viper.GetString("rpc.product.direct_addr"); addr != "" {
		c, err := productservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("create product rpc client failed (direct): %w", err))
		}
		return c
	}
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("create etcd resolver failed: %w", err))
	}
	c, err := productservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create product rpc client failed: %w", err))
	}
	return c
}

func InitInventoryClient() inventoryservice.Client {
	name := serviceNameOrDefault("rpc.inventory.service_name", "inventory-service")
	if addr := viper.GetString("rpc.inventory.direct_addr"); addr != "" {
		c, err := inventoryservice.NewClient(name, client.WithHostPorts(addr))
		if err != nil {
			panic(fmt.Errorf("create inventory rpc client failed (direct): %w", err))
		}
		return c
	}
	r, err := etcd.NewEtcdResolver(etcdEndpoints())
	if err != nil {
		panic(fmt.Errorf("create etcd resolver failed: %w", err))
	}
	c, err := inventoryservice.NewClient(name, client.WithResolver(r))
	if err != nil {
		panic(fmt.Errorf("create inventory rpc client failed: %w", err))
	}
	return c
}

func serviceNameOrDefault(key, def string) string {
	if name := viper.GetString(key); name != "" {
		return name
	}
	return def
}
