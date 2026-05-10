package ioc

import (
	"fmt"
	"strings"
	"time"

	seckillconfig "github.com/XDWow/DouyinMall/backend/internal/seckill/config"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/mq"
	pushmq "github.com/apache/rocketmq-client-go/v2"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
	pushproducer "github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/spf13/viper"
)

func InitRocketMQConfig() seckillconfig.RocketMQConfig {
	cfg := seckillconfig.RocketMQConfig{
		ProducerGroup:       "seckill-request-producer",
		RequestGroup:        "seckill-request-consumer",
		DeadLetterGroup:     "seckill-request-dead-letter-consumer",
		OrderStatusGroup:    "seckill-order-status-consumer",
		HandleTimeoutSec:    25,
		ProducerMaxAttempts: 3,
		GlobalWorkerNum:     32,
	}
	if err := viper.UnmarshalKey("rocketmq", &cfg); err != nil {
		panic(fmt.Errorf("read rocketmq config failed: %w", err))
	}
	if cfg.NameServer == "" {
		panic("rocketmq.name_server is required")
	}
	cfg.AccessKey = viper.GetString("rocketmq.access_key")
	cfg.SecretKey = viper.GetString("rocketmq.secret_key")
	return cfg
}

func resolveRocketMQNameServers(cfg seckillconfig.RocketMQConfig) pushprimitive.NamesrvAddr {
	parts := strings.Split(cfg.NameServer, ",")
	addrs := make([]string, 0, len(parts))
	for _, part := range parts {
		addr := strings.TrimSpace(part)
		if addr == "" {
			continue
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		panic("rocketmq.name_server is required")
	}
	return pushprimitive.NamesrvAddr(addrs)
}

func withPushCredentials(opts []pushconsumer.Option, cfg seckillconfig.RocketMQConfig) []pushconsumer.Option {
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		opts = append(opts, pushconsumer.WithCredentials(pushprimitive.Credentials{
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		}))
	}
	return opts
}

func InitRocketMQProducerClient(listener *mq.TransactionListener) pushmq.TransactionProducer {
	cfg := InitRocketMQConfig()
	opts := []pushproducer.Option{
		pushproducer.WithNameServer(resolveRocketMQNameServers(cfg)),
		pushproducer.WithGroupName(cfg.ProducerGroup),
		pushproducer.WithRetry(int(cfg.ProducerMaxAttempts)),
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		opts = append(opts, pushproducer.WithCredentials(pushprimitive.Credentials{
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		}))
	}

	producer, err := pushmq.NewTransactionProducer(listener, opts...)
	if err != nil {
		panic(fmt.Errorf("init rocketmq producer failed: %w", err))
	}
	if err = producer.Start(); err != nil {
		panic(fmt.Errorf("start rocketmq producer failed: %w", err))
	}
	return producer
}

func InitSeckillPushConsumer() pushmq.PushConsumer {
	cfg := InitRocketMQConfig()
	opts := []pushconsumer.Option{
		pushconsumer.WithNameServer(resolveRocketMQNameServers(cfg)),
		pushconsumer.WithGroupName(cfg.RequestGroup),
		pushconsumer.WithConsumeGoroutineNums(cfg.GlobalWorkerNum),
		pushconsumer.WithPullBatchSize(1),
		pushconsumer.WithConsumeMessageBatchMaxSize(1),
	}
	opts = withPushCredentials(opts, cfg)

	consumer, err := pushmq.NewPushConsumer(opts...)
	if err != nil {
		panic(fmt.Errorf("init seckill push consumer failed: %w", err))
	}
	return consumer
}

func InitSeckillDeadLetterTopic() string {
	cfg := InitRocketMQConfig()
	return mq.NativeDeadLetterTopic(cfg.RequestGroup)
}

func InitSeckillDeadLetterPushConsumer() pushmq.PushConsumer {
	cfg := InitRocketMQConfig()
	opts := []pushconsumer.Option{
		pushconsumer.WithNameServer(resolveRocketMQNameServers(cfg)),
		pushconsumer.WithGroupName(cfg.DeadLetterGroup),
		pushconsumer.WithConsumeGoroutineNums(cfg.GlobalWorkerNum),
		pushconsumer.WithPullBatchSize(1),
		pushconsumer.WithConsumeMessageBatchMaxSize(1),
	}
	opts = withPushCredentials(opts, cfg)

	consumer, err := pushmq.NewPushConsumer(opts...)
	if err != nil {
		panic(fmt.Errorf("init seckill dead-letter push consumer failed: %w", err))
	}
	return consumer
}

func InitSeckillConsumerOptions() mq.SeckillConsumerOptions {
	cfg := InitRocketMQConfig()
	return mq.SeckillConsumerOptions{
		HandleTimeout:   time.Duration(cfg.HandleTimeoutSec) * time.Second,
		GlobalWorkerNum: cfg.GlobalWorkerNum,
	}
}

func InitSeckillOrderStatusPushConsumer() pushmq.PushConsumer {
	cfg := InitRocketMQConfig()
	opts := []pushconsumer.Option{
		pushconsumer.WithNameServer(resolveRocketMQNameServers(cfg)),
		pushconsumer.WithGroupName(cfg.OrderStatusGroup),
		pushconsumer.WithConsumeGoroutineNums(cfg.GlobalWorkerNum),
		pushconsumer.WithPullBatchSize(1),
		pushconsumer.WithConsumeMessageBatchMaxSize(1),
	}
	opts = withPushCredentials(opts, cfg)

	consumer, err := pushmq.NewPushConsumer(opts...)
	if err != nil {
		panic(fmt.Errorf("init seckill order-status push consumer failed: %w", err))
	}
	return consumer
}
