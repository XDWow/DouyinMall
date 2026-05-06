package ioc

import (
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/cart/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
	"github.com/apache/rocketmq-clients/golang/credentials"
	"github.com/spf13/viper"
)

type RocketMQConfig struct {
	Endpoint             string `mapstructure:"endpoint"`
	AccessKey            string `mapstructure:"access_key"`
	SecretKey            string `mapstructure:"secret_key"`
	ConsumerGroup        string `mapstructure:"consumer_group"`
	AwaitDurationSec     int    `mapstructure:"await_duration_sec"`
	MaxMessageNum        int32  `mapstructure:"max_message_num"`
	InvisibleDurationSec int    `mapstructure:"invisible_duration_sec"`
}

func initRocketMQConfig() RocketMQConfig {
	cfg := RocketMQConfig{
		ConsumerGroup:        "cart-order-consumer",
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

func InitRocketMQOrderStatusConsumer() rmq_client.SimpleConsumer {
	cfg := initRocketMQConfig()
	consumer, err := rmq_client.NewSimpleConsumer(
		&rmq_client.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: cfg.ConsumerGroup,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithAwaitDuration(time.Duration(cfg.AwaitDurationSec)*time.Second),
		rmq_client.WithSubscriptionExpressions(map[string]*rmq_client.FilterExpression{
			mq.TopicOrderStatusUpdate: rmq_client.SUB_ALL,
		}),
	)
	if err != nil {
		panic(fmt.Errorf("init rocketmq simple consumer failed: %w", err))
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
