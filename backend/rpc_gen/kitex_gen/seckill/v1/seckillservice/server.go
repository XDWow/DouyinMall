package seckillservice

import (
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	server "github.com/cloudwego/kitex/server"
)

func NewServer(handler seckillv1.SeckillService, opts ...server.Option) server.Server {
	var options []server.Option
	options = append(options, opts...)
	svr := server.NewServer(options...)
	if err := svr.RegisterService(serviceInfo(), handler); err != nil {
		panic(err)
	}
	return svr
}

func RegisterService(svr server.Server, handler seckillv1.SeckillService, opts ...server.RegisterOption) error {
	return svr.RegisterService(serviceInfo(), handler, opts...)
}


