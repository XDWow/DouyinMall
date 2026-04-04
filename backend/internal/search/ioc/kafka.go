package ioc

import (
	"github.com/IBM/sarama"
	"github.com/spf13/viper"
)

// InitKafkaClient 鍒濆鍖?Kafka Client
func InitKafkaClient() sarama.Client {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		// 濡傛灉浠庣幆澧冨彉閲忚鍙栵紙瀛楃涓诧級锛岃浆鎹负鏁扮粍
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
		panic("鍒濆鍖?Kafka Client 澶辫触: " + err.Error())
	}

	return client
}


