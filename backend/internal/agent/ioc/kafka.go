package ioc

import (
	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agent/config"
	"github.com/spf13/viper"
)

// InitKafkaClient 初始化 Kafka Client（producer + consumer 共用）
func InitKafkaClient() sarama.Client {
	c := config.KafkaConfig{
		Brokers:                []string{"localhost:9092"},
		ProducerRetryMax:       3,
		ConsumerOffsetsInitial: "newest",
	}
	_ = viper.UnmarshalKey("kafka", &c)

	brokers := c.Brokers
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}

	scfg := sarama.NewConfig()
	// Producer：安全关键配置硬编码，重试次数由外部调优
	scfg.Producer.Return.Successes = true
	scfg.Producer.RequiredAcks = sarama.WaitForAll // 强一致，不允许配置降级
	scfg.Producer.Retry.Max = c.ProducerRetryMax
	scfg.Producer.Idempotent = true // 幂等写入，防重复
	scfg.Net.MaxOpenRequests = 1    // idempotent 要求

	// Consumer
	scfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	scfg.Consumer.Offsets.Initial = offsetInitialFromString(c.ConsumerOffsetsInitial)

	client, err := sarama.NewClient(brokers, scfg)
	if err != nil {
		panic("初始化 Kafka Client 失败: " + err.Error())
	}
	return client
}

// InitKafkaSyncProducer 初始化同步生产者
func InitKafkaSyncProducer(client sarama.Client) sarama.SyncProducer {
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		panic("初始化 Kafka SyncProducer 失败: " + err.Error())
	}
	return producer
}

// offsetInitialFromString 将配置字符串转换为 sarama offset 常量
// 支持 "oldest" / "newest"（默认）
func offsetInitialFromString(s string) int64 {
	if s == "oldest" {
		return sarama.OffsetOldest
	}
	return sarama.OffsetNewest
}
