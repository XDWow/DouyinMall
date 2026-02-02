package ioc

import (
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	etcd "github.com/kitex-contrib/registry-etcd"
)

// 创建订单服务RPC客户端
// 用于库存服务同步回调订单服务更新状态
func NewOrderClient() orderservice.Client {
	r, err := etcd.NewEtcdResolver([]string{"localhost:12379"})
	if err != nil {
		klog.Fatal("创建etcd resolver失败", err)
	}

	cli, err := orderservice.NewClient(
		"order",
		client.WithResolver(r),
	)
	if err != nil {
		klog.Fatal("创建order client失败", err)
	}

	return cli
}
