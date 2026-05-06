package rocketmqx

import (
	"context"
	"time"

	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type SimpleConsumer interface {
	Start() error
	GracefulStop() error
	Ack(ctx context.Context, messageView *rmq_client.MessageView) error
	Receive(ctx context.Context, maxMessageNum int32, invisibleDuration time.Duration) ([]*rmq_client.MessageView, error)
	ChangeInvisibleDuration(messageView *rmq_client.MessageView, invisibleDuration time.Duration) error
}

type ConsumeHandler interface {
	Consume(ctx context.Context, consumer SimpleConsumer, msgs []*rmq_client.MessageView) error
}

type MessageProducer interface {
	Send(ctx context.Context, msg *rmq_client.Message) error
	SendBatch(ctx context.Context, msgs []*rmq_client.Message) []error
	GracefulStop() error
}
