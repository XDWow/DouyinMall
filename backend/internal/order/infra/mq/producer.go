package mq

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const (
	topicUpdateOrderStatus = "order_status_update"
)

type OrderStatusProducer interface {
	SendMessage(ctx context.Context, evt domain.OrderStatusUpdateEvent) error
	SendMessages(ctx context.Context, events []domain.OrderStatusUpdateEvent) []error
}

type Producer struct {
	producer rocketmqx.MessageProducer
}

func NewProducer(producer rocketmqx.MessageProducer) OrderStatusProducer {
	return &Producer{producer: producer}
}

func (p *Producer) SendMessage(ctx context.Context, evt domain.OrderStatusUpdateEvent) error {
	data, _ := json.Marshal(evt)
	msg := &rmq_client.Message{
		Topic: topicUpdateOrderStatus,
		Body:  data,
	}
	msg.SetKeys(int64Key(evt.OrderID))
	return p.producer.Send(ctx, msg)
}

func (p *Producer) SendMessages(ctx context.Context, events []domain.OrderStatusUpdateEvent) []error {
	if len(events) == 0 {
		return nil
	}

	msgs := make([]*rmq_client.Message, 0, len(events))
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		msg := &rmq_client.Message{
			Topic: topicUpdateOrderStatus,
			Body:  data,
		}
		msg.SetKeys(int64Key(evt.OrderID))
		msgs = append(msgs, msg)
	}
	return p.producer.SendBatch(ctx, msgs)
}

func int64Key(orderID int64) string {
	return strconv.FormatInt(orderID, 10)
}
