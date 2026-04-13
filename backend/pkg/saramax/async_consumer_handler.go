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
	return nil
}

func (h AsyncHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 异步消费、批量提交 offset：先在一段时间窗口内收集一批消息，再 errgroup 并发处理，最后统一 MarkMessage。
func (h AsyncHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ch := claim.Messages()
	batchsize := h.batchsize
	for {
		var eg errgroup.Group
		msgs := make([]*sarama.ConsumerMessage, 0, batchsize)
		// 防止一直凑不够一批导致无法提交 offset。
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := false
		for i := 0; i < batchsize && !done; i++ {
			select {
			case <-ctx.Done():
				done = true
			case msg, ok := <-ch:
				if !ok {
					cancel()
					return nil
				}
				msgs = append(msgs, msg)
				eg.Go(func() error {
					var err error
					var t T
					if err = json.Unmarshal(msg.Value, &t); err != nil {
						h.l.Error("反序列化消息体失败",
							logger.String("topic", msg.Topic),
							logger.Int32("partition", msg.Partition),
							logger.Int64("offset", msg.Offset),
							logger.Error(err))
						return nil
					}

					for i := 0; i < 3; i++ {
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
					// 忽略错误，避免阻塞 errgroup；统计上可再补 metric。
					return nil
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

// NewAsyncHandler 构造异步 Handler；batchsize 为每批最多拉取条数（再并发处理）。
func NewAsyncHandler[T any](l logger.LoggerV1, consume func(msg *sarama.ConsumerMessage, t T) error, batchsize int) *AsyncHandler[T] {
	return &AsyncHandler[T]{
		l:         l,
		fn:        consume,
		batchsize: batchsize,
	}
}
