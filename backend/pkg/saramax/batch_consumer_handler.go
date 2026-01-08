package saramax

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/cmd/pkg/logger"
)

type BatchHandler[T any] struct {
	l  logger.LoggerV1
	fn func(msgs []*sarama.ConsumerMessage, ts []T) error
	// 用 option 模式来设置这个 batchSize 和 duration
	batchSize     int
	batchDuration time.Duration

	maxConcurrency int
}

func NewBatchHandler[T any](l logger.LoggerV1, fn func(msgs []*sarama.ConsumerMessage, ts []T) error, batchsize int) *BatchHandler[T] {
	return &BatchHandler[T]{l: l, fn: fn, batchDuration: time.Second, batchSize: 10, maxConcurrency: 16}
}

func (b *BatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b *BatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// 拿消息给你写好了, 提交也帮你写好了, 都是通用的
// 只需传入你的消费信息业务 fn()
// session 是本次消费会话的上下文，负责提交 Offset 和获取组内元信息
// claim 提供分配给当前实例的某个分区的信息和该分区的消息通道
// ConsumeClaim 可以考虑在这个封装里面提供统一的重试机制
// 批量接口
func (h *BatchHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	// 这个可以做成参数
	const batchSize = 10
	for {
		msgs := make([]*sarama.ConsumerMessage, 0, batchSize)
		ts := make([]T, 0, batchSize)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := false
		for i := 0; i < batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				// 这一批次已经超时了，
				// 或者，整个 consumer 被关闭了
				// 不再尝试凑够一批了
				done = true
			case msg, ok := <-msgsCh:
				if !ok {
					cancel()
					// channel 被关闭了
					return nil
				}
				msgs = append(msgs, msg)
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					// 消息格式都不对，没啥好处理的
					// 但是也不能直接返回，在线上的时候要继续处理下去
					h.l.Error("反序列化消息体失败",
						logger.String("topic", msg.Topic),
						logger.Int32("partition", msg.Partition),
						logger.Int64("offset", msg.Offset),
						// 这里也可以考虑打印 msg.Value，但是有些时候 msg 本身也包含敏感数据
						logger.Error(err))
					// 不中断，继续下一个
					session.MarkMessage(msg, "")
					continue
				}
				ts = append(ts, t)
			}
		}
		err := h.fn(msgs, ts)
		if err == nil {
			// 这边就要都提交了
			for _, msg := range msgs {
				session.MarkMessage(msg, "")
			}
		} else {
			// 这里可以考虑重试，也可以在具体的业务逻辑里面重试
			// 也就是 eg.Go 里面重试
		}
		cancel()
	}
}

// 异步消费+批量接口实现，经典的错误，标准的0分
func (b *BatchHandler[T]) ConsumeClaimFalse(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	sem := make(chan struct{}, b.maxConcurrency)

	for {
		// 批量消息数据，要有个超时 context ，避免一直等待凑齐一批消息
		ctx, cancel := context.WithTimeout(context.Background(), b.batchDuration)
		// 未超时
		done := false
		// 初始化消息切片
		msgs := make([]*sarama.ConsumerMessage, 0, b.batchSize)
		ts := make([]T, 0, b.batchSize)
		for i := 0; i < b.batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				done = true
			case msg, ok := <-msgsCh:
				// 通道关闭了,退出消费
				if !ok {
					cancel()
					return nil
				}
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					b.l.Error("反序列化失败",
						logger.Error(err),
						logger.String("topic", msg.Topic),
						logger.Int64("partition", int64(msg.Partition)),
						logger.Int64("offset", msg.Offset))
					// 后面不执行了，跳到下一条消息
					continue
				}
				msgs = append(msgs, msg)
				ts = append(ts, t)
			}
		}
		// 一批数据拿完了
		// 批量消费
		cancel()
		// 一个消息都没拿到，不能执行消耗fn,继续循环等消息吧
		if len(msgs) == 0 {
			continue
		}
		// 控制并发数
		sem <- struct{}{}
		go func(msgs []*sarama.ConsumerMessage, ts []T) {
			defer func() { <-sem }()
			err := b.fn(msgs, ts)
			if err != nil {
				b.l.Error("调用业务批量接口失败",
					logger.Error(err))
				// 你这里整个批次都要记下来

				// 还要继续往前消费
				return
			}
			for _, msg := range msgs {
				// 这样，万无一失
				session.MarkMessage(msg, "")
			}
		}(msgs, ts)
	}
}
