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
	// 鍟ヤ篃涓嶅共
	return nil
}

func (h Handler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	/// 鍟ヤ篃涓嶅共
	return nil
}

// 鍚屾娑堣垂
func (h Handler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgs := claim.Messages()
	for msg := range msgs {
		var t T
		err := json.Unmarshal(msg.Value, &t)
		if err != nil {
			h.l.Error("鍙嶅簭鍒楀寲澶辫触",
				logger.Error(err),
				// 鍑洪敊娑堟伅瀹氫綅:topic + partition + offset
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", int64(msg.Offset)))
			continue
		}
		// 鎷垮埌娑堟伅涔嬪悗锛岃皟鐢ㄨ嚜瀹氫箟鐨?consume 澶勭悊娑堟伅
		// 骞跺湪杩欓噷鎵ц閲嶈瘯
		for i := 0; i < 3; i++ {
			err = h.fn(msg, t)
			if err == nil {
				break
			}
			h.l.Error("澶勭悊娑堟伅澶辫触",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", msg.Offset))
		}

		if err != nil {
			h.l.Error("澶勭悊娑堟伅澶辫触-閲嶈瘯娆℃暟涓婇檺",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int64("partition", int64(msg.Partition)),
				logger.Int64("offset", msg.Offset))
		} else {
			// 澶勭悊瀹屾秷鎭悗锛岃寰楁彁浜?
			session.MarkMessage(msg, "")
		}
	}
	return nil
}

// 浼犲叆瀹炵幇濂界殑鑷畾涔?consume
func NewHandler[T any](l logger.LoggerV1, consume func(msg *sarama.ConsumerMessage, t T) error) *Handler[T] {
	return &Handler[T]{
		l:  l,
		fn: consume,
	}
}


