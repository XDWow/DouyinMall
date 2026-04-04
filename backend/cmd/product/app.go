package main

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/product/producer"
	"github.com/cloudwego/kitex/server"
)

type ProducerComponent interface {
	Start(ctx context.Context) error
}

type producerWrapper struct {
	producer.Producer
}

func (p *producerWrapper) Start(ctx context.Context) error {
	return p.Producer.Start(ctx)
}

type App struct {
	Server    server.Server
	Producers []ProducerComponent
}


