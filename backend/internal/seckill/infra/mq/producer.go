package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

const (
	TopicSeckillCreateOrder           = "seckill.create_order"
	TopicSeckillCreateOrderDeadLetter = "seckill.create_order.dead_letter"
	TopicOrderStatusUpdate            = "order_status_update"
)

type Producer struct{ producer sarama.SyncProducer }

func NewProducer(producer sarama.SyncProducer) domain.Producer {
	return &Producer{producer: producer}
}

func (p *Producer) Publish(ctx context.Context, evt domain.Event) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化秒杀事件失败: %w", err)
	}
	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicSeckillCreateOrder,
		Key:   sarama.StringEncoder(strconv.FormatInt(evt.ActivityID, 10)),
		Value: sarama.ByteEncoder(data),
	})
	if err != nil {
		return fmt.Errorf("发布秒杀事件到 Kafka 失败: %w", err)
	}
	return nil
}

type seckillDeadLetterMessage struct {
	Event           domain.Event `json:"event"`
	Error           string       `json:"error"`
	Attempts        int          `json:"attempts"`
	SourceTopic     string       `json:"source_topic"`
	SourcePartition int32        `json:"source_partition"`
	SourceOffset    int64        `json:"source_offset"`
	FailedAt        time.Time    `json:"failed_at"`
}

func publishSeckillDeadLetter(_ context.Context, producer sarama.SyncProducer, msg seckillDeadLetterMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化秒杀死信失败: %w", err)
	}
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicSeckillCreateOrderDeadLetter,
		Key:   sarama.StringEncoder(strconv.FormatInt(msg.Event.ActivityID, 10)),
		Value: sarama.ByteEncoder(data),
	})
	if err != nil {
		return fmt.Errorf("发布秒杀死信失败: %w", err)
	}
	return nil
}


