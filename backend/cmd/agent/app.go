package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/agent/knowledge"
	"github.com/cloudwego/kitex/server"
)

// App Agent 微服务应用
type App struct {
	Server   server.Server
	Consumer *mq.MessageConsumer
	Indexer  *knowledge.Indexer
}
