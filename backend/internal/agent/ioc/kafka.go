package ioc

import (
	"fmt"

	"github.com/IBM/sarama"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
)

// NewKafkaClient 创建共享 Sarama Client（consumer / producer 共用）。
func NewKafkaClient(cfg agentconfig.KafkaConfig) (sarama.Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers is empty")
	}
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_6_0_0
	saramaCfg.Consumer.Return.Errors = true
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true
	return sarama.NewClient(cfg.Brokers, saramaCfg)
}

// NewKafkaSyncProducer 从 Client 创建同步生产者。
func NewKafkaSyncProducer(client sarama.Client) (sarama.SyncProducer, error) {
	return sarama.NewSyncProducerFromClient(client)
}
