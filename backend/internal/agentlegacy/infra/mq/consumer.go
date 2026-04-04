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

// 閹靛綊鍣哄☉鍫ｅ瀭 Kafka 濞戝牊浼呴獮鎯版儰鎼?
// 閸掆晝鏁?saramax.BatchHandler 鐎圭偟骞?閺冨爼妫?閺佷即鍣?閸欏矁袝閸欐垶澹掗柌蹇旂Х鐠愮櫢绱?
//   - 閸戞垵顧?batchSize 閺夛紕鐝涢崡鍐差槱閻?
//   - 鐡掑懓绻?batchDuration 娑撳秷鍐绘稉鈧幍閫涚瘍缁斿宓嗘径鍕倞
//
// 閸楁洘顐奸幍褰掑櫤濞戝牐鍨傜亸鍡楊樋鏉烆喖顕拠婵堟畱濞戝牊浼呯仦鏇為挬閸氬孩澧界悰灞肩濞?BatchInsertMessages閿?
// 閺勬崘鎲查梽宥勭秵 MySQL 閸愭瑥鍙嗗▎鈩冩殶閿涘本褰佹妯绘殻娴ｆ挸鎮堕崥?
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

// Start 閸氼垰濮╁☉鍫ｅ瀭閼板拑绱欓棃鐐烘▎婵夌儑绱濋崘鍛村劥 goroutine 閼奉亜濮╅柌宥堢箾閿?
func (c *MessageConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient(groupAgentMessages, c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	// batchSize=10閿涙碍鐦￠幍瑙勬付婢舵俺浠涢崥?10 娑?Kafka 濞戝牊浼呴敍鍫㈠ 20 閺?chat message閿涘绱?
	// 娑撯偓濞?BatchInsertMessages 閽€钘夌氨
	handler := saramax.NewBatchHandler[domain.ChatMessageEvent](c.l, c.consume, 10)

	go func() {
		for {
			if err := cg.Consume(context.Background(),
				[]string{TopicAgentMessages}, handler); err != nil {
				c.l.Error("agent 濞戝牊浼呭☉鍫ｅ瀭瀵倸鐖堕敍灞藉祮鐏忓棝鍣告潻?,
					logger.Error(err))
			}
		}
	}()

	return nil
}

// Stop 娴兼﹢娉ら崗鎶芥４濞戝牐鍨傞懓?
func (c *MessageConsumer) Stop() {
	if c.consumerGrp != nil {
		_ = c.consumerGrp.Close()
	}
}

// consume 閹靛綊鍣哄☉鍫ｅ瀭閸ョ偠鐨熼敍姘潔楠炲啿顦挎稉顏冪皑娴犲墎娈戝☉鍫熶紖 閳?娑撯偓濞嗏剝澹掗柌?INSERT
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


