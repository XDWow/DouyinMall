package rocketmqx

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ConsumerOptions struct {
	MaxMessageNum       int32
	InvisibleDuration   time.Duration
	ReceiveErrorBackoff time.Duration
}

func (o ConsumerOptions) withDefaults() ConsumerOptions {
	if o.MaxMessageNum <= 0 {
		o.MaxMessageNum = 16
	}
	if o.InvisibleDuration <= 0 {
		o.InvisibleDuration = 30 * time.Second
	}
	if o.ReceiveErrorBackoff <= 0 {
		o.ReceiveErrorBackoff = time.Second
	}
	return o
}

type Consumer struct {
	consumer SimpleConsumer
	handler  ConsumeHandler
	logger   logger.LoggerV1
	options  ConsumerOptions
	stopCh   chan struct{}
}

func NewConsumer(consumer SimpleConsumer, handler ConsumeHandler, l logger.LoggerV1, options ConsumerOptions) *Consumer {
	return &Consumer{
		consumer: consumer,
		handler:  handler,
		logger:   l,
		options:  options.withDefaults(),
		stopCh:   make(chan struct{}),
	}
}

func (c *Consumer) Start() error {
	if err := c.consumer.Start(); err != nil {
		return err
	}
	go c.loop()
	return nil
}

func (c *Consumer) Stop() error {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
	return c.consumer.GracefulStop()
}

func (c *Consumer) loop() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		msgs, err := c.consumer.Receive(context.Background(), c.options.MaxMessageNum, c.options.InvisibleDuration)
		if err != nil {
			c.logger.Warn("rocketmq receive failed", logger.Error(err))
			select {
			case <-c.stopCh:
				return
			case <-time.After(c.options.ReceiveErrorBackoff):
			}
			continue
		}
		if len(msgs) == 0 {
			continue
		}
		if err = c.handler.Consume(context.Background(), c.consumer, msgs); err != nil {
			c.logger.Warn("rocketmq consume batch failed", logger.Error(err))
		}
	}
}
