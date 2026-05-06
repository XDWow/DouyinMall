package mq

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const TopicPaymentStatusUpdate = "payment_status_update"

type PaymentStatusProducer interface {
	SendMessage(ctx context.Context, evt domain.PaymentStatusUpdateEvent) error
	SendMessages(ctx context.Context, events []domain.PaymentStatusUpdateEvent) []error
}

type Producer struct {
	producer rocketmqx.MessageProducer
}

func NewProducer(producer rocketmqx.MessageProducer) PaymentStatusProducer {
	return &Producer{producer: producer}
}

func (p *Producer) SendMessage(ctx context.Context, evt domain.PaymentStatusUpdateEvent) error {
	data, _ := json.Marshal(evt)
	msg := &rmq_client.Message{
		Topic: TopicPaymentStatusUpdate,
		Body:  data,
	}
	msg.SetKeys(strconv.FormatInt(evt.OrderID, 10))
	return p.producer.Send(ctx, msg)
}

func (p *Producer) SendMessages(ctx context.Context, events []domain.PaymentStatusUpdateEvent) []error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]*rmq_client.Message, 0, len(events))
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		msg := &rmq_client.Message{
			Topic: TopicPaymentStatusUpdate,
			Body:  data,
		}
		msg.SetKeys(strconv.FormatInt(evt.OrderID, 10))
		msgs = append(msgs, msg)
	}
	return p.producer.SendBatch(ctx, msgs)
}
