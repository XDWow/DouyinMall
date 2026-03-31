package mq

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
)

const (
	topicUpdateOrderStatus = "order_status_update"
)

type SaramaProducer struct {
	producer sarama.SyncProducer
}

func NewSaramaProducer(syncProducer sarama.SyncProducer) SaramaProducer {
	return SaramaProducer{
		producer: syncProducer,
	}
}

func (p *SaramaProducer) SendMessage(ctx context.Context, evt domain.OrderStatusUpdateEvent) error {
	data, _ := json.Marshal(evt)
	_, _, err := p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topicUpdateOrderStatus,
		Value: sarama.StringEncoder(data),
	})
	return err
}

func (p *SaramaProducer) SendMessages(ctx context.Context, events []domain.OrderStatusUpdateEvent) []error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]*sarama.ProducerMessage, 0, len(events))
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		messages = append(messages, &sarama.ProducerMessage{
			Topic: topicUpdateOrderStatus,
			Value: sarama.StringEncoder(data),
		})
	}
	return sendMessages(p.producer, messages)
}

func sendMessages(producer sarama.SyncProducer, messages []*sarama.ProducerMessage) []error {
	err := producer.SendMessages(messages)
	if err == nil {
		return nil
	}

	if errs, ok := err.(sarama.ProducerErrors); ok {
		results := make([]error, len(messages))
		for _, e := range errs {
			if e.Msg == nil {
				continue
			}
			for i, msg := range messages {
				if msg == e.Msg {
					results[i] = e.Err
					break
				}
			}
		}
		return results
	}

	results := make([]error, len(messages))
	for i := range results {
		results[i] = err
	}
	return results
}
