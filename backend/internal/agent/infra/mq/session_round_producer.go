package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IBM/sarama"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// SessionRoundProducer 将单轮持久化请求发到 Kafka，由消费者批量写入 MySQL
type SessionRoundProducer struct {
	p     sarama.SyncProducer
	topic string
}

func NewSessionRoundProducer(p sarama.SyncProducer, topic string) *SessionRoundProducer {
	t := strings.TrimSpace(topic)
	if t == "" {
		t = TopicSessionRoundPersist
	}
	return &SessionRoundProducer{p: p, topic: t}
}

// PublishRound 序列化并发送；使用 session_id 作为分区键保证同会话顺序。
func (s *SessionRoundProducer) PublishRound(ctx context.Context, session domain.Session, userMessage, assistantMessage domain.SessionMessage) error {
	_ = ctx
	if s == nil || s.p == nil {
		return fmt.Errorf("session round producer is not configured")
	}
	ev := NewSessionRoundPersistEvent(session, userMessage, assistantMessage)
	if len(ev.Messages) == 0 {
		return nil
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: s.topic,
		Key:   sarama.StringEncoder(session.SessionID),
		Value: sarama.ByteEncoder(payload),
	}
	_, _, err = s.p.SendMessage(msg)
	return err
}
