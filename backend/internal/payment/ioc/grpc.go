package ioc

import (
	"fmt"
	"net"

	"github.com/XDWow/DouyinMall/backend/internal/payment/transport/grpc"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func InitGRPCServer(paymentHandler *grpc.PaymentHandler) server.Server {
	// 鍒濆鍖?etcd 娉ㄥ唽涓績
	endpoints := viper.GetStringSlice("etcd.endpoints")
	// 濡傛灉浠庣幆澧冨彉閲忚鍙栵紙瀛楃涓诧級锛岃浆鎹负鏁扮粍
	if len(endpoints) == 0 {
		if ep := viper.GetString("etcd.endpoints"); ep != "" {
			endpoints = []string{ep}
		}
	}
	r, err := etcd.NewEtcdRegistry(endpoints)
	if err != nil {
		panic(fmt.Errorf("鍒涘缓 etcd 娉ㄥ唽涓績澶辫触: %w", err))
	}

	// 鏈嶅姟閰嶇疆
	port := viper.GetInt("grpc.server.port")
	serviceName := viper.GetString("grpc.server.name")
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))

	// 鍒涘缓 Kitex 鏈嶅姟
	svr := paymentv1.NewServer(
		paymentHandler,
		server.WithRegistry(r),       // 娉ㄥ唽鍒?etcd
		server.WithServiceAddr(addr), // 鏈嶅姟鍦板潃
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	)

	return svr
}


