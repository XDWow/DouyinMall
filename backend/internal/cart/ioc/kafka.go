package ioc

import (
	"github.com/IBM/sarama"
	"github.com/spf13/viper"
)

// InitKafkaClient 鍒濆鍖?Kafka Client锛堢敤浜庢秷璐硅€咃級
func InitKafkaClient() sarama.Client {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		if addr := viper.GetString("kafka.brokers"); addr != "" {
			brokers = []string{addr}
		} else {
			brokers = []string{"localhost:9092"}
		}
	}

	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		panic("鍒濆鍖?Kafka Client 澶辫触: " + err.Error())
	}

	return client
}


