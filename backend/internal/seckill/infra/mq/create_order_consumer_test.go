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

type fakePushConsumer struct {
	subscribeTopic    string
	subscribeSelector pushconsumer.MessageSelector
	callback          func(context.Context, ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error)
	startCalled       bool
	shutdownCalled    bool
	startPanic        any
}

func (f *fakePushConsumer) Start() error {
	if f.startPanic != nil {
		panic(f.startPanic)
	}
	f.startCalled = true
	return nil
}

func (f *fakePushConsumer) Shutdown() error {
	f.shutdownCalled = true
	return nil
}

func (f *fakePushConsumer) Subscribe(
	topic string,
	selector pushconsumer.MessageSelector,
	cb func(context.Context, ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error),
) error {
	f.subscribeTopic = topic
	f.subscribeSelector = selector
	f.callback = cb
	return nil
}

func TestSeckillConsumerSubscribe(t *testing.T) {
	fake := &fakePushConsumer{}
	c := &SeckillConsumer{
		consumer: fake,
		logger:   logger.NewNopLogger(),
		options:  SeckillConsumerOptions{}.withDefaults(),
		process:  func(context.Context, seckilldomain.Event) error { return nil },
	}

	err := c.subscribe()
	require.NoError(t, err)
	require.Equal(t, TopicSeckillRequest, fake.subscribeTopic)
	require.Equal(t, pushconsumer.TAG, fake.subscribeSelector.Type)
	require.Equal(t, "*", fake.subscribeSelector.Expression)
	require.NotNil(t, fake.callback)
}

func TestConsumeMessagesSuccess(t *testing.T) {
	var got seckilldomain.Event
	c := &SeckillConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		process: func(_ context.Context, evt seckilldomain.Event) error {
			got = evt
			return nil
		},
	}

	body, err := json.Marshal(seckilldomain.Event{
		RequestNo:  "req-1",
		ActivityID: 1001,
		UserID:     2002,
	})
	require.NoError(t, err)

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message: pushprimitive.Message{Body: body},
		MsgId:   "msg-1",
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeSuccess, result)
	require.Equal(t, "req-1", got.RequestNo)
	require.EqualValues(t, 1001, got.ActivityID)
}

func TestConsumeMessagesRetryLaterOnProcessError(t *testing.T) {
	c := &SeckillConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		process: func(context.Context, seckilldomain.Event) error {
			return errors.New("boom")
		},
	}

	body, err := json.Marshal(seckilldomain.Event{
		RequestNo:  "req-2",
		ActivityID: 1002,
		UserID:     2003,
	})
	require.NoError(t, err)

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message:        pushprimitive.Message{Body: body},
		MsgId:          "msg-2",
		ReconsumeTimes: 1,
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeRetryLater, result)
}

func TestConsumeMessagesSkipsPoisonMessage(t *testing.T) {
	called := false
	c := &SeckillConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		process: func(context.Context, seckilldomain.Event) error {
			called = true
			return nil
		},
	}

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message: pushprimitive.Message{Body: []byte("{")},
		MsgId:   "bad-msg",
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeSuccess, result)
	require.False(t, called)
}
