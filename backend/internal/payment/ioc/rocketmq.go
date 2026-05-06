package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
	"github.com/apache/rocketmq-clients/golang/credentials"
	"github.com/spf13/viper"
)

type RocketMQConfig struct {
	Endpoint            string `mapstructure:"endpoint"`
	AccessKey           string `mapstructure:"access_key"`
	SecretKey           string `mapstructure:"secret_key"`
	ProducerMaxAttempts int32  `mapstructure:"producer_max_attempts"`
}

func initRocketMQConfig() RocketMQConfig {
	cfg := RocketMQConfig{
		ProducerMaxAttempts: 3,
	}
	if err := viper.UnmarshalKey("rocketmq", &cfg); err != nil {
		panic(fmt.Errorf("read rocketmq config failed: %w", err))
	}
	if cfg.Endpoint == "" {
		panic("rocketmq.endpoint is required")
	}
	return cfg
}

func InitRocketMQProducerClient() rmq_client.Producer {
	cfg := initRocketMQConfig()
	producer, err := rmq_client.NewProducer(
		&rmq_client.Config{
			Endpoint: cfg.Endpoint,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithTopics(mq.TopicPaymentStatusUpdate),
		rmq_client.WithMaxAttempts(cfg.ProducerMaxAttempts),
	)
	if err != nil {
		panic(fmt.Errorf("init rocketmq producer failed: %w", err))
	}
	if err = producer.Start(); err != nil {
		panic(fmt.Errorf("start rocketmq producer failed: %w", err))
	}
	return producer
}

func InitPaymentStatusProducer(producer rmq_client.Producer) rocketmqx.MessageProducer {
	return rocketmqx.NewProducer(producer)
}

func InitPaymentMQProducer(producer rocketmqx.MessageProducer) mq.PaymentStatusProducer {
	return mq.NewProducer(producer)
}
