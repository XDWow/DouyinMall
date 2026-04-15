package ioc

import (
	"strings"

	"github.com/IBM/sarama"
	"github.com/spf13/viper"
)

func InitKafkaClient() sarama.Client {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		if v := viper.GetString("kafka.brokers"); v != "" {
			brokers = strings.Split(v, ",")
		}
	}
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	client, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		panic("初始化 Kafka Client 失败: " + err.Error())
	}
	return client
}

func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic("初始化 Kafka SyncProducer 失败: " + err.Error())
	}
	return producer
}


