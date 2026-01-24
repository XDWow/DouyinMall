package main

import (
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"github.com/cloudwego/kitex/server"
)

type App struct {
	Server    server.Server
	Consumers []saramax.Consumer
}
