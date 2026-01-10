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
	// 啥也不干
	return nil
}

func (h Handler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	/// 啥也不干
	return nil
}

// 同步消费
func (h Handler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgs := claim.Messages()
	for msg := range msgs {
		var t T
		err := json.Unmarshal(msg.Value, &t)
		if err != nil {
			h.l.Error("反序列化失败",
				logger.Error(err),
				// 出错消息定位:topic + partition + offset
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", int64(msg.Offset)))
			continue
		}
		// 拿到消息之后，调用自定义的 consume 处理消息
		// 并在这里执行重试
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
			h.l.Error("处理消息失败-重试次数上限",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", msg.Offset))
		} else {
			// 处理完消息后，记得提交
			session.MarkMessage(msg, "")
		}
	}
	return nil
}

// 传入实现好的自定义 consume
func NewHandler[T any](l logger.LoggerV1, consume func(msg *sarama.ConsumerMessage, t T) error) *Handler[T] {
	return &Handler[T]{
		l:  l,
		fn: consume,
	}
}
