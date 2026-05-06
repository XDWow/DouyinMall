package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/pool"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const (
	TopicSeckillRequest     = "seckill_request"
	TopicOrderStatusUpdate  = "order_status_update"

	defaultGlobalWorkerNum   = 32
	defaultPerActivityLimit  = 8
	defaultReceiveMaxMessage = 16
	defaultShutdownTimeout   = 10 * time.Second
)

func NativeDeadLetterTopic(consumerGroup string) string {
	return "%DLQ%" + consumerGroup
}

type simpleConsumer interface {
	Start() error
	GracefulStop() error
	Receive(ctx context.Context, maxMessageNum int32, invisibleDuration time.Duration) ([]*rmq_client.MessageView, error)
	Ack(ctx context.Context, messageView *rmq_client.MessageView) error
}

type SeckillConsumerOptions struct {
	InvisibleDuration      time.Duration
	HandleTimeout          time.Duration
	ShutdownTimeout        time.Duration
	MaxMessageNum          int32
	GlobalWorkerNum        int
	PerActivityConcurrency int
}

func (o SeckillConsumerOptions) withDefaults() SeckillConsumerOptions {
	if o.InvisibleDuration <= 0 {
		o.InvisibleDuration = 30 * time.Second
	}
	if o.HandleTimeout <= 0 {
		o.HandleTimeout = o.InvisibleDuration - 5*time.Second
		if o.HandleTimeout <= 0 {
			o.HandleTimeout = o.InvisibleDuration / 2
		}
		if o.HandleTimeout <= 0 {
			o.HandleTimeout = time.Second
		}
	}
	if o.ShutdownTimeout <= 0 {
		o.ShutdownTimeout = defaultShutdownTimeout
	}
	if o.MaxMessageNum <= 0 {
		o.MaxMessageNum = defaultReceiveMaxMessage
	}
	if o.GlobalWorkerNum <= 0 {
		o.GlobalWorkerNum = defaultGlobalWorkerNum
	}
	if o.PerActivityConcurrency <= 0 {
		o.PerActivityConcurrency = defaultPerActivityLimit
	}
	return o
}

type SeckillConsumer struct {
	consumer  simpleConsumer
	processor *EventProcessor
	logger    logger.LoggerV1
	options   SeckillConsumerOptions

	taskPool *pool.KeyedConsumerPool
	stopCh   chan struct{}
	cancel   context.CancelFunc
}

type seckillMessageTask struct {
	msg *rmq_client.MessageView
	evt seckilldomain.Event
}

func NewSeckillConsumer(consumer simpleConsumer, processor *EventProcessor, l logger.LoggerV1, options SeckillConsumerOptions) *SeckillConsumer {
	c := &SeckillConsumer{
		consumer:  consumer,
		processor: processor,
		logger:    l,
		options:   options.withDefaults(),
		stopCh:    make(chan struct{}),
	}
	taskPool, err := pool.NewKeyedConsumerPool(pool.KeyedConsumerPoolOptions{
		GlobalWorkerNum:        c.options.GlobalWorkerNum,
		PerActivityConcurrency: c.options.PerActivityConcurrency,
		TaskTimeout:            c.options.HandleTimeout,
	}, l)
	if err != nil {
		panic(fmt.Errorf("init seckill keyed consumer pool failed: %w", err))
	}
	c.taskPool = taskPool
	return c
}

func (c *SeckillConsumer) Start() error {
	if err := c.consumer.Start(); err != nil {
		return err
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.loop(loopCtx)
	return nil
}

func (c *SeckillConsumer) Stop() error {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.taskPool != nil {
		if ok := c.taskPool.CloseWithTimeout(c.options.ShutdownTimeout); !ok {
			c.logger.Warn("seckill consumer stopped before all tasks finished",
				logger.Field{Key: "shutdownTimeout", Value: c.options.ShutdownTimeout})
		}
	}
	if c.consumer != nil {
		return c.consumer.GracefulStop()
	}
	return nil
}

func (c *SeckillConsumer) loop(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := c.consumer.Receive(ctx, c.options.MaxMessageNum, c.options.InvisibleDuration)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Warn("receive seckill message failed", logger.Error(err))
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if err = c.dispatchMessage(msg); err != nil {
				c.logger.Warn("dispatch seckill message rejected",
					logger.Error(err),
					logger.String("messageID", msg.GetMessageId()),
					logger.Int32("deliveryAttempt", msg.GetDeliveryAttempt()))
			}
		}
	}
}

func (c *SeckillConsumer) dispatchMessage(msg *rmq_client.MessageView) error {
	var evt seckilldomain.Event
	if err := json.Unmarshal(msg.GetBody(), &evt); err != nil {
		return fmt.Errorf("decode seckill message failed: %w", err)
	}

	return c.taskPool.Submit(context.Background(), activityKey(evt.ActivityID), func(ctx context.Context) error {
		return c.handleTask(ctx, seckillMessageTask{
			msg: msg,
			evt: evt,
		})
	})
}

func (c *SeckillConsumer) handleTask(ctx context.Context, messageTask seckillMessageTask) error {
	err := c.processor.Process(ctx, messageTask.evt)
	if err == nil {
		if ackErr := c.consumer.Ack(ctx, messageTask.msg); ackErr != nil {
			c.logger.Error("ack seckill message failed", logger.Error(ackErr))
		}
		return nil
	}

	c.logger.Warn("seckill message will retry",
		logger.Error(err),
		logger.String("requestNo", messageTask.evt.RequestNo),
		logger.Int32("deliveryAttempt", messageTask.msg.GetDeliveryAttempt()))
	return err
}

func activityKey(activityID int64) string {
	return strconv.FormatInt(activityID, 10)
}
