package ioc

import (
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
	"github.com/apache/rocketmq-clients/golang/credentials"
	"github.com/spf13/viper"
)

type RocketMQConfig struct {
	Endpoint             string `mapstructure:"endpoint"`
	AccessKey            string `mapstructure:"access_key"`
	SecretKey            string `mapstructure:"secret_key"`
	ProducerMaxAttempts  int32  `mapstructure:"producer_max_attempts"`
	PaymentConsumerGroup string `mapstructure:"payment_consumer_group"`
	AwaitDurationSec     int    `mapstructure:"await_duration_sec"`
	MaxMessageNum        int32  `mapstructure:"max_message_num"`
	InvisibleDurationSec int    `mapstructure:"invisible_duration_sec"`
}

func initRocketMQConfig() RocketMQConfig {
	cfg := RocketMQConfig{
		ProducerMaxAttempts:  3,
		PaymentConsumerGroup: "order-payment-consumer",
		AwaitDurationSec:     5,
		MaxMessageNum:        16,
		InvisibleDurationSec: 30,
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
		rmq_client.WithTopics("order_status_update"),
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

func InitOrderStatusProducer(producer rmq_client.Producer) rocketmqx.MessageProducer {
	return rocketmqx.NewProducer(producer)
}

func InitOrderMQProducer(producer rocketmqx.MessageProducer) mq.OrderStatusProducer {
	return mq.NewProducer(producer)
}

func InitPaymentStatusConsumerClient() rmq_client.SimpleConsumer {
	cfg := initRocketMQConfig()
	consumer, err := rmq_client.NewSimpleConsumer(
		&rmq_client.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: cfg.PaymentConsumerGroup,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithAwaitDuration(time.Duration(cfg.AwaitDurationSec)*time.Second),
		rmq_client.WithSubscriptionExpressions(map[string]*rmq_client.FilterExpression{
			mq.TopicPaymentStatusUpdate: rmq_client.SUB_ALL,
		}),
	)
	if err != nil {
		panic(fmt.Errorf("init payment status simple consumer failed: %w", err))
	}
	return consumer
}

func InitRocketMQConsumerOptions() rocketmqx.ConsumerOptions {
	cfg := initRocketMQConfig()
	return rocketmqx.ConsumerOptions{
		MaxMessageNum:     cfg.MaxMessageNum,
		InvisibleDuration: time.Duration(cfg.InvisibleDurationSec) * time.Second,
	}
}
