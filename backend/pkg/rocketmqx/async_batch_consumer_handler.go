package rocketmqx

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type AsyncBatchHandler[T any] struct {
	logger logger.LoggerV1
	fn     func(ctx context.Context, msgs []*rmq_client.MessageView, ts []T) error
}

func NewAsyncBatchHandler[T any](l logger.LoggerV1, fn func(ctx context.Context, msgs []*rmq_client.MessageView, ts []T) error) *AsyncBatchHandler[T] {
	return &AsyncBatchHandler[T]{
		logger: l,
		fn:     fn,
	}
}

func (h *AsyncBatchHandler[T]) Consume(ctx context.Context, consumer SimpleConsumer, msgs []*rmq_client.MessageView) error {
	validMsgs := make([]*rmq_client.MessageView, 0, len(msgs))
	values := make([]T, 0, len(msgs))
	for _, msg := range msgs {
		var t T
		if err := json.Unmarshal(msg.GetBody(), &t); err != nil {
			h.logger.Error("rocketmq async batch unmarshal failed",
				logger.Error(err),
				logger.String("topic", msg.GetTopic()),
				logger.String("messageID", msg.GetMessageId()))
			h.ack(ctx, consumer, msg)
			continue
		}
		validMsgs = append(validMsgs, msg)
		values = append(values, t)
	}
	if len(validMsgs) == 0 {
		return nil
	}
	if err := h.fn(ctx, validMsgs, values); err != nil {
		return err
	}
	for _, msg := range validMsgs {
		h.ack(ctx, consumer, msg)
	}
	return nil
}

func (h *AsyncBatchHandler[T]) ack(ctx context.Context, consumer SimpleConsumer, msg *rmq_client.MessageView) {
	if err := consumer.Ack(ctx, msg); err != nil {
		h.logger.Error("rocketmq async batch ack failed",
			logger.Error(err),
			logger.String("topic", msg.GetTopic()),
			logger.String("messageID", msg.GetMessageId()))
	}
}
