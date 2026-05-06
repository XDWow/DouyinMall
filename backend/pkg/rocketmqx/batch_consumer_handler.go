package rocketmqx

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type BatchHandler[T any] struct {
	logger logger.LoggerV1
	fn     func(msgs []*rmq_client.MessageView, ts []T) error
}

func NewBatchHandler[T any](l logger.LoggerV1, fn func(msgs []*rmq_client.MessageView, ts []T) error) *BatchHandler[T] {
	return &BatchHandler[T]{
		logger: l,
		fn:     fn,
	}
}

func (h *BatchHandler[T]) Consume(ctx context.Context, consumer SimpleConsumer, msgs []*rmq_client.MessageView) error {
	validMsgs := make([]*rmq_client.MessageView, 0, len(msgs))
	values := make([]T, 0, len(msgs))
	for _, msg := range msgs {
		var t T
		if err := json.Unmarshal(msg.GetBody(), &t); err != nil {
			h.logger.Error("rocketmq batch unmarshal failed",
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
	if err := h.fn(validMsgs, values); err != nil {
		return err
	}
	for _, msg := range validMsgs {
		h.ack(ctx, consumer, msg)
	}
	return nil
}

func (h *BatchHandler[T]) ack(ctx context.Context, consumer SimpleConsumer, msg *rmq_client.MessageView) {
	if err := consumer.Ack(ctx, msg); err != nil {
		h.logger.Error("rocketmq batch ack failed",
			logger.Error(err),
			logger.String("topic", msg.GetTopic()),
			logger.String("messageID", msg.GetMessageId()))
	}
}
