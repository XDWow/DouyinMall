package main

import (
	"github.com/cloudwego/kitex/server"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)


type App struct {
	Server    server.Server
	Consumers []saramax.Consumer
}
