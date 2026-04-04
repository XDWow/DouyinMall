//go:build legacy_agent

package mq

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// TopicAgentMessages Kafka topic閿涙艾顕拠婵囩Х閹垰绱撳銉﹀瘮娑斿懎瀵?const TopicAgentMessages = "agent_chat_messages"

// MessageProducer 鐏忓棙鐦℃潪顔碱嚠鐠囨繃绉烽幁顖涘闁帒鍩?Kafka閿涘本娴涙禒锝呭斧閺夈儳娈?goroutine 閻╂潙鍟?MySQL
// 娴ｈ法鏁?SyncProducer閿涘牆鍞寸純?3 濞嗭繝鍣哥拠鏇礆閿涘本瀵?session_id 閸掑棗灏穱婵婄槈閸氬奔绱扮拠婵囩Х閹垱婀佹惔?
type MessageProducer struct {
	producer sarama.SyncProducer
}

func NewMessageProducer(producer sarama.SyncProducer) *MessageProducer {
	return &MessageProducer{producer: producer}
}

// ProduceMessages 閹舵洟鈧帗婀版潪顔芥煀濞戝牊浼呴崚?Kafka
// Key = nil 閳?鏉烆喛顕楅崚鍡楀隘閿涘本褰佹妯鸿嫙鐞涘苯瀹抽敍宀勪缉閸忓秶鍎归悙閫涚窗鐠囨繂顕遍懛鏉戝瀻閸栧搫鈧偓鏋?// 濞戝牊浼呴張顒冮煩鐢?CreatedAt 閺冨爼妫块幋绛圭礉閺屻儴顕楅弮鑸靛瘻閺冨爼妫块幒鎺戠碍閸楀啿褰叉穱婵婄槈閺堝绨?func (p *MessageProducer) ProduceMessages(ctx context.Context, event domain.ChatMessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicAgentMessages,
		Key:   nil, // 鏉烆喛顕楅崚鍡楀隘閿涘本褰佹妯烘偠閸?
		Value: sarama.ByteEncoder(data),
	})
	return err
}


