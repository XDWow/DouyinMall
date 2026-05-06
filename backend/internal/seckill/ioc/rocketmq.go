package ioc

import (
	"fmt"
	"strings"
	"time"

	seckillconfig "github.com/XDWow/DouyinMall/backend/internal/seckill/config"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
	"github.com/apache/rocketmq-clients/golang/credentials"
	"github.com/spf13/viper"
)

func InitRocketMQConfig() seckillconfig.RocketMQConfig {
	cfg := seckillconfig.RocketMQConfig{
		RequestGroup:           "seckill-request-consumer",
		DeadLetterGroup:        "seckill-request-dead-letter-consumer",
		InvisibleDurationSec:   30,
		HandleTimeoutSec:       25,
		ShutdownTimeoutSec:     10,
		AwaitDurationSec:       5,
		MaxMessageNum:          16,
		ProducerMaxAttempts:    3,
		GlobalWorkerNum:        32,
		PerActivityConcurrency: 8,
	}
	if err := viper.UnmarshalKey("rocketmq", &cfg); err != nil {
		panic(fmt.Errorf("read rocketmq config failed: %w", err))
	}
	if cfg.Endpoint == "" {
		panic("rocketmq.endpoint is required")
	}
	cfg.AccessKey = viper.GetString("rocketmq.access_key")
	cfg.SecretKey = viper.GetString("rocketmq.secret_key")
	return cfg
}

func InitRocketMQProducerClient(cache seckilldomain.Cache, l logger.LoggerV1) rmq_client.Producer {
	cfg := InitRocketMQConfig()
	producer, err := rmq_client.NewProducer(
		&rmq_client.Config{
			Endpoint: cfg.Endpoint,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithTransactionChecker(mq.NewTransactionChecker(cache, l)),
		rmq_client.WithTopics(mq.TopicSeckillRequest),
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

func InitSeckillSimpleConsumer() rmq_client.SimpleConsumer {
	cfg := InitRocketMQConfig()
	consumer, err := rmq_client.NewSimpleConsumer(
		&rmq_client.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: cfg.RequestGroup,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithAwaitDuration(time.Duration(cfg.AwaitDurationSec)*time.Second),
		rmq_client.WithSubscriptionExpressions(map[string]*rmq_client.FilterExpression{
			mq.TopicSeckillRequest: rmq_client.SUB_ALL,
		}),
	)
	if err != nil {
		panic(fmt.Errorf("init seckill simple consumer failed: %w", err))
	}
	return consumer
}

func InitSeckillDeadLetterSimpleConsumer() rmq_client.SimpleConsumer {
	cfg := InitRocketMQConfig()
	consumer, err := rmq_client.NewSimpleConsumer(
		&rmq_client.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: cfg.DeadLetterGroup,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		rmq_client.WithAwaitDuration(time.Duration(cfg.AwaitDurationSec)*time.Second),
		rmq_client.WithSubscriptionExpressions(map[string]*rmq_client.FilterExpression{
			mq.NativeDeadLetterTopic(cfg.RequestGroup): rmq_client.SUB_ALL,
		}),
	)
	if err != nil {
		if strings.Contains(err.Error(), "TOPIC_NOT_FOUND") {
			fmt.Printf("warning: seckill native DLQ topic %s not found, dead-letter consumer disabled until RocketMQ creates it\n", mq.NativeDeadLetterTopic(cfg.RequestGroup))
			return nil
		}
		panic(fmt.Errorf("init seckill dead-letter consumer failed: %w", err))
	}
	return consumer
}

func InitSeckillConsumerOptions() mq.SeckillConsumerOptions {
	cfg := InitRocketMQConfig()
	return mq.SeckillConsumerOptions{
		InvisibleDuration:      time.Duration(cfg.InvisibleDurationSec) * time.Second,
		HandleTimeout:          time.Duration(cfg.HandleTimeoutSec) * time.Second,
		ShutdownTimeout:        time.Duration(cfg.ShutdownTimeoutSec) * time.Second,
		MaxMessageNum:          cfg.MaxMessageNum,
		GlobalWorkerNum:        cfg.GlobalWorkerNum,
		PerActivityConcurrency: cfg.PerActivityConcurrency,
	}
}

func InitSeckillOrderStatusSimpleConsumer() rmq_client.SimpleConsumer {
	cfg := InitRocketMQConfig()
	group := viper.GetString("rocketmq.order_status_group")
	if group == "" {
		group = "seckill-order-status-consumer"
	}
	consumer, err := rmq_client.NewSimpleConsumer(
		&rmq_client.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: group,
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
		panic(fmt.Errorf("init seckill order status consumer failed: %w", err))
	}
	return consumer
}

func InitSeckillOrderStatusConsumerOptions() rocketmqx.ConsumerOptions {
	cfg := InitRocketMQConfig()
	return rocketmqx.ConsumerOptions{
		MaxMessageNum:     cfg.MaxMessageNum,
		InvisibleDuration: time.Duration(cfg.InvisibleDurationSec) * time.Second,
	}
}
