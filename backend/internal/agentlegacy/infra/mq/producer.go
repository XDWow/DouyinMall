//go:build legacy_agent

package mq

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// TopicAgentMessages Kafka topic锛氬璇濇秷鎭紓姝ユ寔涔呭寲
const TopicAgentMessages = "agent_chat_messages"

// MessageProducer 灏嗘瘡杞璇濇秷鎭姇閫掑埌 Kafka锛屾浛浠ｅ師鏉ョ殑 goroutine 鐩村啓 MySQL
// 浣跨敤 SyncProducer锛堝唴缃?3 娆￠噸璇曪級锛屾寜 session_id 鍒嗗尯淇濊瘉鍚屼細璇濇秷鎭湁搴?
type MessageProducer struct {
	producer sarama.SyncProducer
}

func NewMessageProducer(producer sarama.SyncProducer) *MessageProducer {
	return &MessageProducer{producer: producer}
}

// ProduceMessages 鎶曢€掓湰杞柊娑堟伅鍒?Kafka
// Key = nil 鈫?杞鍒嗗尯锛屾彁楂樺苟琛屽害锛岄伩鍏嶇儹鐐逛細璇濆鑷村垎鍖哄€炬枩
// 娑堟伅鏈韩甯?CreatedAt 鏃堕棿鎴筹紝鏌ヨ鏃舵寜鏃堕棿鎺掑簭鍗冲彲淇濊瘉鏈夊簭
func (p *MessageProducer) ProduceMessages(ctx context.Context, event domain.ChatMessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicAgentMessages,
		Key:   nil, // 杞鍒嗗尯锛屾彁楂樺悶鍚?
		Value: sarama.ByteEncoder(data),
	})
	return err
}
