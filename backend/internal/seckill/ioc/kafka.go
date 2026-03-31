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
		panic(err)
	}
	return client
}

func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic(err)
	}
	return producer
}
