package mq

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// TopicAgentMessages Kafka topic：对话消息异步持久化
const TopicAgentMessages = "agent_chat_messages"

// MessageProducer 将每轮对话消息投递到 Kafka，替代原来的 goroutine 直写 MySQL
// 使用 SyncProducer（内置 3 次重试），按 session_id 分区保证同会话消息有序
type MessageProducer struct {
	producer sarama.SyncProducer
}

func NewMessageProducer(producer sarama.SyncProducer) *MessageProducer {
	return &MessageProducer{producer: producer}
}

// ProduceMessages 投递本轮新消息到 Kafka
// Key = nil → 轮询分区，提高并行度，避免热点会话导致分区倾斜
// 消息本身带 CreatedAt 时间戳，查询时按时间排序即可保证有序
func (p *MessageProducer) ProduceMessages(ctx context.Context, event domain.ChatMessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicAgentMessages,
		Key:   nil, // 轮询分区，提高吞吐
		Value: sarama.ByteEncoder(data),
	})
	return err
}
