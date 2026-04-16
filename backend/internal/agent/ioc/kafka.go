package ioc

import (
	"fmt"

	"github.com/IBM/sarama"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
)

func NewKafkaClient(cfg agentconfig.KafkaConfig) (sarama.Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("没有 kafka 实例")
	}
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_6_0_0
	saramaCfg.Consumer.Return.Errors = true
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true
	return sarama.NewClient(cfg.Brokers, saramaCfg)
}

func NewKafkaSyncProducer(client sarama.Client) (sarama.SyncProducer, error) {
	return sarama.NewSyncProducerFromClient(client)
}
