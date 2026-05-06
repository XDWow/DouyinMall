package rocketmqx

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type Handler[T any] struct {
	logger       logger.LoggerV1
	fn           func(msg *rmq_client.MessageView, t T) error
	localRetries int
}

func NewHandler[T any](l logger.LoggerV1, consume func(msg *rmq_client.MessageView, t T) error) *Handler[T] {
	return &Handler[T]{
		logger:       l,
		fn:           consume,
		localRetries: 3,
	}
}

func (h *Handler[T]) Consume(ctx context.Context, consumer SimpleConsumer, msgs []*rmq_client.MessageView) error {
	var lastErr error
	for _, msg := range msgs {
		var t T
		if err := json.Unmarshal(msg.GetBody(), &t); err != nil {
			h.logger.Error("rocketmq message unmarshal failed",
				logger.Error(err),
				logger.String("topic", msg.GetTopic()),
				logger.String("messageID", msg.GetMessageId()))
			h.ack(ctx, consumer, msg)
			continue
		}

		var err error
		for i := 0; i < h.localRetries; i++ {
			err = h.fn(msg, t)
			if err == nil {
				break
			}
			h.logger.Error("rocketmq message handling failed",
				logger.Error(err),
				logger.String("topic", msg.GetTopic()),
				logger.String("messageID", msg.GetMessageId()),
				logger.Int("attempt", i+1))
		}
		if err != nil {
			lastErr = err
			h.logger.Error("rocketmq message handling exhausted retries",
				logger.Error(err),
				logger.String("topic", msg.GetTopic()),
				logger.String("messageID", msg.GetMessageId()))
			continue
		}
		h.ack(ctx, consumer, msg)
	}
	return lastErr
}

func (h *Handler[T]) ack(ctx context.Context, consumer SimpleConsumer, msg *rmq_client.MessageView) {
	if err := consumer.Ack(ctx, msg); err != nil {
		h.logger.Error("rocketmq ack failed",
			logger.Error(err),
			logger.String("topic", msg.GetTopic()),
			logger.String("messageID", msg.GetMessageId()))
	}
}
