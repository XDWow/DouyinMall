package mq

import (
	"context"
	"errors"

	"github.com/IBM/sarama"
	"gorm.io/gorm"

	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const (
	TopicSessionRoundPersist        = "agent-session-round-persist"
	GroupSessionRoundPersistDefault = "agent-session-round-consumer"
)

// SessionRoundConsumer 异步批量消费会话轮次落库事件。
type SessionRoundConsumer struct {
	client    sarama.Client
	db        *gorm.DB
	l         logger.LoggerV1
	topic     string
	group     string
	batchSize int

	consumerGrp sarama.ConsumerGroup
}

func NewSessionRoundConsumer(client sarama.Client, db *gorm.DB, l logger.LoggerV1, topic, group string, batchSize int) *SessionRoundConsumer {
	if batchSize <= 0 {
		batchSize = 32
	}
	t := topic
	if t == "" {
		t = TopicSessionRoundPersist
	}
	g := group
	if g == "" {
		g = GroupSessionRoundPersistDefault
	}
	return &SessionRoundConsumer{
		client:    client,
		db:        db,
		l:         l,
		topic:     t,
		group:     g,
		batchSize: batchSize,
	}
}

// Start 在后台 goroutine 中消费；失败会循环重试 Consume。
func (c *SessionRoundConsumer) Start() error {
	if c == nil || c.client == nil || c.db == nil {
		return errors.New("session round consumer: missing client or db")
	}
	cg, err := sarama.NewConsumerGroupFromClient(c.group, c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	inner := saramax.NewBatchHandler[SessionRoundPersistEvent](c.l, c.consumeBatch, c.batchSize)
	handler := &saramax.AsyncBatchHandlerDelegate[SessionRoundPersistEvent]{Inner: inner}
	go func() {
		for {
			if err := cg.Consume(context.Background(), []string{c.topic}, handler); err != nil {
				if c.l != nil {
					c.l.Error("agent session round consumer stopped with error",
						logger.Error(err),
						logger.String("topic", c.topic),
						logger.String("group", c.group))
				}
			}
		}
	}()
	return nil
}

func (c *SessionRoundConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}

func (c *SessionRoundConsumer) consumeBatch(_ []*sarama.ConsumerMessage, events []SessionRoundPersistEvent) error {
	if len(events) == 0 {
		return nil
	}
	ctx := context.Background()
	items := SessionRoundEventsToRepoBatch(events)
	return agentrepository.BatchPersistSessionRounds(ctx, c.db, items)
}
