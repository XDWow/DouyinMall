package saramax

import (
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type Handler[T any] struct {
	l  logger.LoggerV1
	fn func(msg *sarama.ConsumerMessage, t T) error
}

func (h Handler[T]) Setup(session sarama.ConsumerGroupSession) error {
	// sarama 生命周期钩子，此处无额外逻辑。
	return nil
}

func (h Handler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 同步逐条消费：反序列化后调用业务 fn，带最多 3 次重试，成功后提交 offset。
func (h Handler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgs := claim.Messages()
	for msg := range msgs {
		var t T
		err := json.Unmarshal(msg.Value, &t)
		if err != nil {
			h.l.Error("反序列化消息体失败",
				logger.Error(err),
				// 出错消息定位：topic + partition + offset
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", int64(msg.Offset)))
			continue
		}
		// 拿到消息后调用自定义 consume；在此循环内做重试。
		for i := 0; i < 3; i++ {
			err = h.fn(msg, t)
			if err == nil {
				break
			}
			h.l.Error("处理消息失败",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", msg.Offset))
		}

		if err != nil {
			h.l.Error("处理消息失败-已达重试上限",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", msg.Offset))
		} else {
			session.MarkMessage(msg, "")
		}
	}
	return nil
}

// NewHandler 构造同步 Handler，consume 签名为 (原始消息, 已反序列化的业务体)。
func NewHandler[T any](l logger.LoggerV1, consume func(msg *sarama.ConsumerMessage, t T) error) *Handler[T] {
	return &Handler[T]{
		l:  l,
		fn: consume,
	}
}
