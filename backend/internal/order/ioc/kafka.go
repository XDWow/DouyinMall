package ioc

import (
	"github.com/IBM/sarama"
	"github.com/spf13/viper"
)

// InitKafkaClient 初始化 Kafka Client
func InitKafkaClient() sarama.Client {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		// 若配置为单个 broker 字符串
		if addr := viper.GetString("kafka.brokers"); addr != "" {
			brokers = []string{addr}
		} else {
			brokers = []string{"localhost:9092"}
		}
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		panic("初始化 Kafka Client 失败: " + err.Error())
	}

	return client
}

// InitKafkaSyncProducer 初始化同步 Producer
func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic("初始化 Kafka SyncProducer 失败: " + err.Error())
	}

	return producer
}


