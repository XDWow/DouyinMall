package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
)

const (
	TopicSeckillRequest    = "seckill_request"
	TopicOrderStatusUpdate = "order_status_update"

	defaultGlobalWorkerNum = 32
	defaultHandleTimeout   = 25 * time.Second
)

func NativeDeadLetterTopic(consumerGroup string) string {
	return "%DLQ%" + consumerGroup
}

type pushConsumer interface {
	Start() error
	Shutdown() error
	Subscribe(topic string, selector pushconsumer.MessageSelector,
		f func(context.Context, ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error)) error
}

type SeckillConsumerOptions struct {
	HandleTimeout   time.Duration
	GlobalWorkerNum int
}

func (o SeckillConsumerOptions) withDefaults() SeckillConsumerOptions {
	if o.HandleTimeout <= 0 {
		o.HandleTimeout = defaultHandleTimeout
	}
	if o.GlobalWorkerNum <= 0 {
		o.GlobalWorkerNum = defaultGlobalWorkerNum
	}
	return o
}

type SeckillConsumer struct {
	consumer pushConsumer
	logger   logger.LoggerV1
	options  SeckillConsumerOptions
	process  func(context.Context, seckilldomain.Event) error
}

func NewSeckillConsumer(consumer pushConsumer, processor *EventProcessor, l logger.LoggerV1, options SeckillConsumerOptions) *SeckillConsumer {
	options = options.withDefaults()
	c := &SeckillConsumer{
		consumer: consumer,
		logger:   l,
		options:  options,
	}
	if processor != nil {
		c.process = processor.Process
	}
	if c.process == nil {
		panic("seckill processor is required")
	}
	if c.consumer != nil {
		if err := c.subscribe(); err != nil {
			panic(fmt.Errorf("subscribe seckill push consumer failed: %w", err))
		}
	}
	return c
}

func (c *SeckillConsumer) subscribe() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Subscribe(TopicSeckillRequest, pushconsumer.MessageSelector{
		Type:       pushconsumer.TAG,
		Expression: "*",
	}, c.consumeMessages)
}

func (c *SeckillConsumer) Start() error {
	if c.consumer == nil {
		return nil
	}
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("seckill request push consumer started",
		logger.Int("consumeConcurrency", c.options.GlobalWorkerNum),
		logger.Field{Key: "handleTimeout", Value: c.options.HandleTimeout},
		logger.String("consumerMode", "push"))
	return nil
}

func (c *SeckillConsumer) Stop() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Shutdown()
}

func (c *SeckillConsumer) consumeMessages(ctx context.Context, msgs ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error) {
	for _, msg := range msgs {
		var evt seckilldomain.Event
		if err := json.Unmarshal(msg.Body, &evt); err != nil {
			c.logger.Error("decode seckill message failed, skip poison message",
				logger.Error(err),
				logger.String("messageID", msg.MsgId))
			continue
		}

		if err := c.handleEventWithTimeout(ctx, evt); err != nil {
			c.logger.Warn("process seckill message failed, waiting for MQ retry",
				logger.Error(err),
				logger.String("requestNo", evt.RequestNo),
				logger.String("messageID", msg.MsgId),
				logger.Int32("reconsumeTimes", msg.ReconsumeTimes))
			return pushconsumer.ConsumeRetryLater, nil
		}
	}
	return pushconsumer.ConsumeSuccess, nil
}

func (c *SeckillConsumer) handleEventWithTimeout(ctx context.Context, evt seckilldomain.Event) error {
	taskCtx := ctx
	cancel := func() {}
	if c.options.HandleTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, c.options.HandleTimeout)
	}
	defer cancel()
	return c.process(taskCtx, evt)
}
