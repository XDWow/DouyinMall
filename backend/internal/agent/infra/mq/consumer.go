package mq

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"gorm.io/gorm"
)

const groupAgentMessages = "agent-message-consumer"

// 批量消费 Kafka 消息并落库
// 利用 saramax.BatchHandler 实现"时间+数量"双触发批量消费：
//   - 凑够 batchSize 条立即处理
//   - 超过 batchDuration 不足一批也立即处理
//
// 单次批量消费将多轮对话的消息展平后执行一次 BatchInsertMessages，
// 显著降低 MySQL 写入次数，提高整体吞吐
type MessageConsumer struct {
	client sarama.Client
	db     *gorm.DB
	l      logger.LoggerV1

	consumerGrp sarama.ConsumerGroup
}

func NewMessageConsumer(
	client sarama.Client,
	db *gorm.DB,
	l logger.LoggerV1,
) *MessageConsumer {
	return &MessageConsumer{client: client, db: db, l: l}
}

// Start 启动消费者（非阻塞，内部 goroutine 自动重连）
func (c *MessageConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient(groupAgentMessages, c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	// batchSize=10：每批最多聚合 10 个 Kafka 消息（约 20 条 chat message），
	// 一次 BatchInsertMessages 落库
	handler := saramax.NewBatchHandler[domain.ChatMessageEvent](c.l, c.consume, 10)

	go func() {
		for {
			if err := cg.Consume(context.Background(),
				[]string{TopicAgentMessages}, handler); err != nil {
				c.l.Error("agent 消息消费异常，即将重连",
					logger.Error(err))
			}
		}
	}()

	return nil
}

// Stop 优雅关闭消费者
func (c *MessageConsumer) Stop() {
	if c.consumerGrp != nil {
		_ = c.consumerGrp.Close()
	}
}

// consume 批量消费回调：展平多个事件的消息 → 一次批量 INSERT
func (c *MessageConsumer) consume(
	_ []*sarama.ConsumerMessage,
	events []domain.ChatMessageEvent,
) error {
	var allMsgs []agentdb.Message
	for _, evt := range events {
		for _, m := range evt.Messages {
			allMsgs = append(allMsgs, agentdb.Message{
				SessionID:  m.SessionID,
				Role:       string(m.Role),
				Content:    m.Content,
				Intent:     int8(m.Intent),
				Confidence: m.Confidence,
				TokensUsed: m.TokensUsed,
				LatencyMs:  int(m.LatencyMs),
			})
		}
	}
	if len(allMsgs) == 0 {
		return nil
	}
	return c.db.Create(&allMsgs).Error
}
