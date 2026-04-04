package ioc

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/spf13/viper"
)

// InitKafkaClient 鍒濆鍖?Kafka Client锛堟秷璐硅€呴渶瑕侊級
func InitKafkaClient() sarama.Client {
	// 榛樿閰嶇疆
	c := config.KafkaConfig{
		Brokers: []string{"localhost:19092"},
	}
	err := viper.UnmarshalKey("kafka", &c)
	if err != nil {
		panic(fmt.Errorf("Kafka閰嶇疆璇诲彇澶辫触: %w", err))
	}

	// Kafka 閰嶇疆
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true

	// 鍒涘缓 Kafka Client
	client, err := sarama.NewClient(c.Brokers, cfg)
	if err != nil {
		panic(fmt.Errorf("Kafka Client鍒涘缓澶辫触: %w", err))
	}

	return client
}

// InitKafkaSyncProducer 鍒濆鍖?Kafka SyncProducer锛堝鏋滃皢鏉ラ渶瑕佸彂閫佹淇￠槦鍒楁椂浣跨敤锛?
func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic(fmt.Errorf("Kafka SyncProducer鍒涘缓澶辫触: %w", err))
	}
	return producer
}


