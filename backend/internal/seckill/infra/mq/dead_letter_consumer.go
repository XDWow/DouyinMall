package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
)

type DeadLetterConsumer struct {
	consumer pushConsumer
	topic    string
	process  func(context.Context, seckilldomain.DeadLetterEvent) error
	logger   logger.LoggerV1
	options  SeckillConsumerOptions
}

func NewDeadLetterConsumer(consumer pushConsumer, deadLetterTopic string, processor *EventProcessor, l logger.LoggerV1, options SeckillConsumerOptions) *DeadLetterConsumer {
	options = options.withDefaults()
	c := &DeadLetterConsumer{
		consumer: consumer,
		topic:    deadLetterTopic,
		logger:   l,
		options:  options,
	}
	if processor != nil {
		c.process = processor.ProcessDeadLetter
	}
	if c.process == nil {
		panic("dead-letter processor is required")
	}
	if c.consumer != nil {
		if err := c.subscribe(); err != nil {
			panic(fmt.Errorf("subscribe seckill dead-letter push consumer failed: %w", err))
		}
	}
	return c
}

func (c *DeadLetterConsumer) subscribe() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Subscribe(c.topic, pushconsumer.MessageSelector{
		Type:       pushconsumer.TAG,
		Expression: "*",
	}, c.consumeMessages)
}

func (c *DeadLetterConsumer) Start() (err error) {
	if c.consumer == nil {
		c.logger.Warn("seckill dead-letter consumer disabled, DLQ topic not configured")
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			_ = c.consumer.Shutdown()
			c.logger.Warn("seckill dead-letter consumer disabled because native DLQ topic is unavailable",
				logger.String("topic", c.topic),
				logger.Field{Key: "panic", Value: fmt.Sprint(r)})
			err = nil
		}
	}()
	if err := c.consumer.Start(); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "TOPIC_NOT_FOUND") ||
			strings.Contains(errMsg, "route info not found") ||
			strings.Contains(errMsg, "it may not exist") {
			_ = c.consumer.Shutdown()
			c.logger.Warn("seckill native DLQ topic not found, dead-letter consumer stays disabled",
				logger.Error(err),
				logger.String("topic", c.topic))
			return nil
		}
		return err
	}
	c.logger.Info("seckill dead-letter push consumer started",
		logger.String("topic", c.topic),
		logger.Int("consumeConcurrency", c.options.GlobalWorkerNum),
		logger.Field{Key: "handleTimeout", Value: c.options.HandleTimeout})
	return nil
}

func (c *DeadLetterConsumer) Stop() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Shutdown()
}

func (c *DeadLetterConsumer) consumeMessages(ctx context.Context, msgs ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error) {
	for _, msg := range msgs {
		var evt seckilldomain.Event
		if err := json.Unmarshal(msg.Body, &evt); err != nil {
			c.logger.Error("decode dead-letter seckill message failed, skip poison message",
				logger.Error(err),
				logger.String("messageID", msg.MsgId))
			continue
		}

		dead := seckilldomain.DeadLetterEvent{
			Event:           evt,
			SourceMessageID: msg.MsgId,
			DeliveryAttempt: msg.ReconsumeTimes,
		}
		if err := c.handleDeadLetterWithTimeout(ctx, dead); err != nil {
			c.logger.Warn("process seckill dead-letter message failed, waiting for MQ retry",
				logger.Error(err),
				logger.String("requestNo", evt.RequestNo),
				logger.String("messageID", msg.MsgId),
				logger.Int32("reconsumeTimes", msg.ReconsumeTimes))
			return pushconsumer.ConsumeRetryLater, nil
		}
	}
	return pushconsumer.ConsumeSuccess, nil
}

func (c *DeadLetterConsumer) handleDeadLetterWithTimeout(ctx context.Context, dead seckilldomain.DeadLetterEvent) error {
	taskCtx := ctx
	cancel := func() {}
	if c.options.HandleTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, c.options.HandleTimeout)
	}
	defer cancel()
	return c.process(taskCtx, dead)
}
