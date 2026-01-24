package mq

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
)

const topicUpdateOrderStatus = "order_status_update"

type SaramaProducer struct {
	producer sarama.SyncProducer
}

func NewSaramaProducer(client sarama.Client, nodeID string) (*SaramaProducer, error) {
	p, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}
	return &SaramaProducer{
		producer: p,
	}, nil
}

func (p *SaramaProducer) SendMessage(ctx context.Context, evt domain.OrderStatusUpdateEvent) error {
	data, _ := json.Marshal(evt)
	_, _, err := p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topicUpdateOrderStatus,
		Value: sarama.StringEncoder(data),
	})
	return err
}

// SendMessages 批量发送消息（性能优化在发送层，保持消息独立性）
// 返回每个消息的发送结果，失败被隔离而不是放大
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

	err := p.producer.SendMessages(messages) // mq批量发送api，以下三种结果：
	// 全部成功
	if err == nil {
		return nil
	}

	if errs, ok := err.(sarama.ProducerErrors); ok {
		// 部分失败或全部失败
		results := make([]error, len(events))
		for _, e := range errs {
			if e.Msg != nil {
				for i, msg := range messages {
					if msg == e.Msg {
						results[i] = e.Err
						break
					}
				}
			}
		}
		return results
	}

	// 可能 kafka 都挂了，根本没发（非sarama.ProducerErrors），导致的全部失败
	results := make([]error, len(events))
	for i := range results {
		results[i] = err
	}
	return results
}
