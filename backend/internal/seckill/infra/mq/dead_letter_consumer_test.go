package mq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/stretchr/testify/require"
)

func TestDeadLetterConsumerSubscribe(t *testing.T) {
	fake := &fakePushConsumer{}
	c := &DeadLetterConsumer{
		consumer: fake,
		topic:    NativeDeadLetterTopic("seckill-request-consumer"),
		logger:   logger.NewNopLogger(),
		options:  SeckillConsumerOptions{}.withDefaults(),
		process:  func(context.Context, seckilldomain.DeadLetterEvent) error { return nil },
	}

	err := c.subscribe()
	require.NoError(t, err)
	require.Equal(t, NativeDeadLetterTopic("seckill-request-consumer"), fake.subscribeTopic)
	require.Equal(t, pushconsumer.TAG, fake.subscribeSelector.Type)
	require.Equal(t, "*", fake.subscribeSelector.Expression)
	require.NotNil(t, fake.callback)
}

func TestDeadLetterConsumerRetryLaterOnProcessError(t *testing.T) {
	c := &DeadLetterConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		process: func(context.Context, seckilldomain.DeadLetterEvent) error {
			return errors.New("boom")
		},
	}

	body, err := json.Marshal(seckilldomain.Event{
		RequestNo:  "req-dead-1",
		ActivityID: 1001,
		UserID:     2001,
	})
	require.NoError(t, err)

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message:        pushprimitive.Message{Body: body},
		MsgId:          "dead-msg-1",
		ReconsumeTimes: 2,
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeRetryLater, result)
}

func TestDeadLetterConsumerSkipsPoisonMessage(t *testing.T) {
	called := false
	c := &DeadLetterConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		process: func(context.Context, seckilldomain.DeadLetterEvent) error {
			called = true
			return nil
		},
	}

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message: pushprimitive.Message{Body: []byte("{")},
		MsgId:   "bad-dead-msg",
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeSuccess, result)
	require.False(t, called)
}

func TestDeadLetterConsumerStartRecoversFromMissingDLQTopicPanic(t *testing.T) {
	fake := &fakePushConsumer{startPanic: "the topic=%DLQ%seckill-request-consumer route info not found, it may not exist"}
	c := &DeadLetterConsumer{
		consumer: fake,
		topic:    NativeDeadLetterTopic("seckill-request-consumer"),
		logger:   logger.NewNopLogger(),
		options:  SeckillConsumerOptions{}.withDefaults(),
		process:  func(context.Context, seckilldomain.DeadLetterEvent) error { return nil },
	}

	err := c.Start()

	require.NoError(t, err)
	require.True(t, fake.shutdownCalled)
}
