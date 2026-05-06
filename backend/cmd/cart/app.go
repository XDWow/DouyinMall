package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/cart/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
)

type App struct {
	Server        server.Server
	HTTPServer    *ginx.Server
	OrderConsumer *mq.OrderConsumer
}
