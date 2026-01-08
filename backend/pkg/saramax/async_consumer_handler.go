package saramax

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"golang.org/x/sync/errgroup"
)

type AsyncHandler[T any] struct {
	l         logger.LoggerV1
	fn        func(msg *sarama.ConsumerMessage, t T) error
	batchsize int
}

func (h AsyncHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	// 啥也不干
	return nil
}

func (h AsyncHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	/// 啥也不干
	return nil
}

// 异步消费，批量提交
func (h AsyncHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ch := claim.Messages()
	batchsize := h.batchsize
	for {
		var eg errgroup.Group
		msgs := make([]*sarama.ConsumerMessage, 0, batchsize)
		// 防止一直凑不够一批,无法提交
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := false
		for i := 0; i < batchsize && !done; i++ {
			select {
			case <-ctx.Done():
				done = true
			case msg, ok := <-ch:
				if !ok { // channel 被关闭了
					cancel()
					return nil
				}
				msgs = append(msgs, msg)
				// 异步处理消息
				eg.Go(func() error {
					var err error
					var t T
					if err = json.Unmarshal(msg.Value, &t); err != nil {
						// 消息格式都不对，没啥好处理的
						// 但是也不能直接返回，在线上的时候要继续处理下去
						h.l.Error("反序列化消息体失败",
							logger.String("topic", msg.Topic),
							logger.Int32("partition", msg.Partition),
							logger.Int64("offset", msg.Offset),
							// 这里也可以考虑打印 msg.Value，但是有些时候 msg 本身也包含敏感数据
							logger.Error(err))
						// 不中断，继续下一个
						return nil
					}

					for i := 0; i < 3; i++ { // 重试机制
						err = h.fn(msg, t)
						if err == nil {
							break
						}
					}
					if err != nil {
						h.l.Error("消息消费失败",
							logger.String("topic", msg.Topic),
							logger.Int32("partition", msg.Partition),
							logger.Int64("offset", msg.Offset))
					}
					return nil // 忽略错误，少一个计数没关系
				})
			}
		}
		_ = eg.Wait()
		for _, msg := range msgs {
			session.MarkMessage(msg, "")
		}
		cancel()
	}
}

// 传入实现好的自定义 consume
func NewAsyncHandler[T any](l logger.LoggerV1, consume func(msg *sarama.ConsumerMessage, t T) error, batchsize int) *AsyncHandler[T] {
	return &AsyncHandler[T]{
		l:         l,
		fn:        consume,
		batchsize: batchsize,
	}
}
