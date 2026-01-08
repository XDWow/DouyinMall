package events

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
)

type Producer interface {
	ProduceInconsistentEvent(ctx context.Context, event InconsistentEvent) error
}

type SaramaProducer struct {
	p     sarama.SyncProducer
	topic string
}

func NewSaramaProducer(p sarama.SyncProducer, topic string) Producer {
	return &SaramaProducer{p: p, topic: topic}
}

func (s *SaramaProducer) ProduceInconsistentEvent(ctx context.Context,
	event InconsistentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, _, err = s.p.SendMessage(&sarama.ProducerMessage{
		Topic: s.topic,
		Value: sarama.ByteEncoder(data),
	})
	return err
}
