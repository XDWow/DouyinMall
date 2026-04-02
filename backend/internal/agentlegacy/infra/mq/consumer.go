//go:build legacy_agent

package mq

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agentlegacy/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"gorm.io/gorm"
)

const groupAgentMessages = "agent-message-consumer"

// 鎵归噺娑堣垂 Kafka 娑堟伅骞惰惤搴?
// 鍒╃敤 saramax.BatchHandler 瀹炵幇"鏃堕棿+鏁伴噺"鍙岃Е鍙戞壒閲忔秷璐癸細
//   - 鍑戝 batchSize 鏉＄珛鍗冲鐞?
//   - 瓒呰繃 batchDuration 涓嶈冻涓€鎵逛篃绔嬪嵆澶勭悊
//
// 鍗曟鎵归噺娑堣垂灏嗗杞璇濈殑娑堟伅灞曞钩鍚庢墽琛屼竴娆?BatchInsertMessages锛?
// 鏄捐憲闄嶄綆 MySQL 鍐欏叆娆℃暟锛屾彁楂樻暣浣撳悶鍚?
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

// Start 鍚姩娑堣垂鑰咃紙闈為樆濉烇紝鍐呴儴 goroutine 鑷姩閲嶈繛锛?
func (c *MessageConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient(groupAgentMessages, c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	// batchSize=10锛氭瘡鎵规渶澶氳仛鍚?10 涓?Kafka 娑堟伅锛堢害 20 鏉?chat message锛夛紝
	// 涓€娆?BatchInsertMessages 钀藉簱
	handler := saramax.NewBatchHandler[domain.ChatMessageEvent](c.l, c.consume, 10)

	go func() {
		for {
			if err := cg.Consume(context.Background(),
				[]string{TopicAgentMessages}, handler); err != nil {
				c.l.Error("agent 娑堟伅娑堣垂寮傚父锛屽嵆灏嗛噸杩?,
					logger.Error(err))
			}
		}
	}()

	return nil
}

// Stop 浼橀泤鍏抽棴娑堣垂鑰?
func (c *MessageConsumer) Stop() {
	if c.consumerGrp != nil {
		_ = c.consumerGrp.Close()
	}
}

// consume 鎵归噺娑堣垂鍥炶皟锛氬睍骞冲涓簨浠剁殑娑堟伅 鈫?涓€娆℃壒閲?INSERT
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
