package mq

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/pool"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type DeadLetterConsumer struct {
	consumer  simpleConsumer
	processor *EventProcessor
	logger    logger.LoggerV1
	options   SeckillConsumerOptions

	taskPool *pool.KeyedConsumerPool
	stopCh   chan struct{}
	cancel   context.CancelFunc
}

type deadLetterTask struct {
	msg  *rmq_client.MessageView
	dead seckilldomain.DeadLetterEvent
}

func NewDeadLetterConsumer(consumer simpleConsumer, processor *EventProcessor, l logger.LoggerV1, options SeckillConsumerOptions) *DeadLetterConsumer {
	c := &DeadLetterConsumer{
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
		panic(err)
	}
	c.taskPool = taskPool
	return c
}

func (c *DeadLetterConsumer) Start() error {
	if c.consumer == nil {
		c.logger.Warn("seckill dead-letter consumer disabled because native DLQ topic is not ready")
		return nil
	}
	if err := c.consumer.Start(); err != nil {
		if strings.Contains(err.Error(), "TOPIC_NOT_FOUND") {
			c.logger.Warn("seckill native DLQ topic is not ready, dead-letter consumer disabled",
				logger.Error(err))
			return nil
		}
		return err
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.loop(loopCtx)
	return nil
}

func (c *DeadLetterConsumer) Stop() error {
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
			c.logger.Warn("seckill dead-letter consumer stopped before all tasks finished",
				logger.Field{Key: "shutdownTimeout", Value: c.options.ShutdownTimeout})
		}
	}
	if c.consumer != nil {
		return c.consumer.GracefulStop()
	}
	return nil
}

func (c *DeadLetterConsumer) loop(ctx context.Context) {
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
			c.logger.Warn("receive seckill dead-letter failed", logger.Error(err))
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if err = c.dispatchMessage(msg); err != nil {
				c.logger.Warn("dispatch seckill dead-letter rejected",
					logger.Error(err),
					logger.String("messageID", msg.GetMessageId()),
					logger.Int32("deliveryAttempt", msg.GetDeliveryAttempt()))
			}
		}
	}
}

func (c *DeadLetterConsumer) dispatchMessage(msg *rmq_client.MessageView) error {
	var evt seckilldomain.Event
	if err := json.Unmarshal(msg.GetBody(), &evt); err != nil {
		return c.consumer.Ack(context.Background(), msg)
	}
	dead := seckilldomain.DeadLetterEvent{
		Event:           evt,
		SourceMessageID: msg.GetMessageId(),
		DeliveryAttempt: msg.GetDeliveryAttempt(),
	}
	return c.taskPool.Submit(context.Background(), activityKey(evt.ActivityID), func(ctx context.Context) error {
		return c.handleTask(ctx, deadLetterTask{
			msg:  msg,
			dead: dead,
		})
	})
}

func (c *DeadLetterConsumer) handleTask(ctx context.Context, deadTask deadLetterTask) error {
	if err := c.processor.ProcessDeadLetter(ctx, deadTask.dead); err != nil {
		c.logger.Warn("dead-letter processing will retry",
			logger.Error(err),
			logger.String("requestNo", deadTask.dead.Event.RequestNo))
		return err
	}
	if err := c.consumer.Ack(ctx, deadTask.msg); err != nil {
		c.logger.Error("ack seckill dead-letter failed", logger.Error(err))
		return err
	}
	return nil
}
