package ioc

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/spf13/viper"
)

// InitKafkaClient 初始化 Kafka Client（消费者需要）
func InitKafkaClient() sarama.Client {
	// 默认配置
	c := config.KafkaConfig{
		Brokers: []string{"localhost:19092"},
	}
	err := viper.UnmarshalKey("kafka", &c)
	if err != nil {
		panic(fmt.Errorf("Kafka配置读取失败: %w", err))
	}

	// Kafka 配置
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true

	// 创建 Kafka Client
	client, err := sarama.NewClient(c.Brokers, cfg)
	if err != nil {
		panic(fmt.Errorf("Kafka Client创建失败: %w", err))
	}

	return client
}

// InitKafkaSyncProducer 初始化 Kafka SyncProducer（如果将来需要发送死信队列时使用）
func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic(fmt.Errorf("Kafka SyncProducer创建失败: %w", err))
	}
	return producer
}
