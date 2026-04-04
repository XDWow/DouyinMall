package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/cart/infra/mq"
	"github.com/cloudwego/kitex/server"
)

type App struct {
	Server        server.Server
	OrderConsumer *mq.OrderConsumer
}


