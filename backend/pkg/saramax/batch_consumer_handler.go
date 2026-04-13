package saramax

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type BatchHandler[T any] struct {
	l  logger.LoggerV1
	fn func(msgs []*sarama.ConsumerMessage, ts []T) error
	// 以下字段由 NewBatchHandler 注入，ConsumeClaimFalse 使用 batchSize / batchDuration / maxConcurrency。
	batchSize     int
	batchDuration time.Duration

	maxConcurrency int
}

func NewBatchHandler[T any](l logger.LoggerV1, fn func(msgs []*sarama.ConsumerMessage, ts []T) error, batchSize int) *BatchHandler[T] {
	if batchSize <= 0 {
		batchSize = 10
	}
	return &BatchHandler[T]{
		l:              l,
		fn:             fn,
		batchDuration:  time.Second,
		batchSize:      batchSize,
		maxConcurrency: 16,
	}
}

func (b *BatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b *BatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 同步批量：攒够一批或超时后调用 fn，成功后统一 MarkMessage。
// session 为本次消费会话上下文，负责提交 offset；claim 为当前分区的消息通道。
func (h *BatchHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	batchSize := h.batchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	bd := h.batchDuration
	if bd <= 0 {
		bd = time.Second
	}
	for {
		msgs := make([]*sarama.ConsumerMessage, 0, batchSize)
		ts := make([]T, 0, batchSize)
		ctx, cancel := context.WithTimeout(context.Background(), bd)
		done := false
		for i := 0; i < batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				// 本批次已超时，或 consumer 正在关闭，不再等待凑满整批。
				done = true
			case msg, ok := <-msgsCh:
				if !ok {
					cancel()
					return nil
				}
				msgs = append(msgs, msg)
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					// 消息体格式非法，没有好的补救方式；线上需继续处理后续消息并推进 offset。
					h.l.Error("反序列化消息体失败",
						logger.String("topic", msg.Topic),
						logger.Int32("partition", msg.Partition),
						logger.Int64("offset", msg.Offset),
						logger.Error(err))
					session.MarkMessage(msg, "")
					continue
				}
				ts = append(ts, t)
			}
		}
		err := h.fn(msgs, ts)
		if err == nil {
			for _, msg := range msgs {
				session.MarkMessage(msg, "")
			}
		}
		// err != nil 时由业务层重试策略处理；此处不提交 offset，便于再次消费。
		cancel()
	}
}

// ConsumeClaimFalse 异步批量：在 goroutine 中调用 fn，不阻塞拉取；适合 IO 较重的批量落库。
// 注意：业务失败时当前实现仍会 MarkMessage，避免 poison message 卡死分区（与常见“至少一次”策略一致）。
func (b *BatchHandler[T]) ConsumeClaimFalse(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	sem := make(chan struct{}, b.maxConcurrency)

	bd := b.batchDuration
	if bd <= 0 {
		bd = time.Second
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), bd)
		done := false
		msgs := make([]*sarama.ConsumerMessage, 0, b.batchSize)
		ts := make([]T, 0, b.batchSize)
		for i := 0; i < b.batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				done = true
			case msg, ok := <-msgsCh:
				if !ok {
					cancel()
					return nil
				}
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					b.l.Error("反序列化消息体失败",
						logger.Error(err),
						logger.String("topic", msg.Topic),
						logger.Int64("partition", int64(msg.Partition)),
						logger.Int64("offset", msg.Offset))
					session.MarkMessage(msg, "")
					continue
				}
				msgs = append(msgs, msg)
				ts = append(ts, t)
			}
		}
		cancel()
		if len(msgs) == 0 {
			continue
		}
		sem <- struct{}{}
		go func(msgs []*sarama.ConsumerMessage, ts []T) {
			defer func() { <-sem }()
			err := b.fn(msgs, ts)
			if err != nil {
				b.l.Error("批量消费业务处理失败",
					logger.Error(err))
			}
			for _, msg := range msgs {
				session.MarkMessage(msg, "")
			}
		}(msgs, ts)
	}
}

// AsyncBatchHandlerDelegate 将 ConsumeClaim 转接到 ConsumeClaimFalse，便于作为 sarama.ConsumerGroupHandler 传入。
type AsyncBatchHandlerDelegate[T any] struct {
	Inner *BatchHandler[T]
}

func (h *AsyncBatchHandlerDelegate[T]) Setup(s sarama.ConsumerGroupSession) error {
	if h.Inner == nil {
		return nil
	}
	return h.Inner.Setup(s)
}

func (h *AsyncBatchHandlerDelegate[T]) Cleanup(s sarama.ConsumerGroupSession) error {
	if h.Inner == nil {
		return nil
	}
	return h.Inner.Cleanup(s)
}

func (h *AsyncBatchHandlerDelegate[T]) ConsumeClaim(s sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	if h.Inner == nil {
		return nil
	}
	return h.Inner.ConsumeClaimFalse(s, claim)
}
