package saramax

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"math/rand"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const (
	// DEFAULT_JITTER_FACTOR 默认抖动因子，用于缓解多 worker 同时唤醒的惊群。
	// 抖动后时长约为 base * (1 ± DEFAULT_JITTER_FACTOR)，例如 1s ± 10% ≈ [0.9s, 1.1s]。
	DEFAULT_JITTER_FACTOR = 0.1
)

// task 包一层便于以后扩展字段（如重试次数、时间戳等）。
type task struct{ msg *sarama.ConsumerMessage }

// calculateJitter 计算带抖动的等待时长：base * (1 + jitter)，jitter ∈ [-jitterFactor, +jitterFactor]。
// jitterFactor 建议约 0.05–0.2（5%–20%）。
func calculateJitter(base time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return base
	}
	jitter := (rand.Float64()*2 - 1) * jitterFactor
	return time.Duration(float64(base) * (1.0 + jitter))
}

// AsyncBatchHandler 泛型异步批量消费：每条 Kafka 消息进 worker，在 worker 内聚合成批后调用 fn。
// T：业务消息体类型。
// fn：批量处理函数，接收 context 与一批反序列化后的 T。
// batchSize：单 worker 聚合条数上限。
// batchDuration：聚合窗口时长，避免无限等待凑批。
// maxConcurrency：worker 数量。
// l：日志。
// localRetries：单批 fn 失败时的本地快速重试次数。
// retryBackoff：重试基准间隔（用于指数退避）。
// shutdownWait：session 关闭 / rebalance 时主协程等待 worker / ack 的上限（当前 ConsumeClaim 快路径未用满该字段）。
type AsyncBatchHandler[T any] struct {
	fn             func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error
	batchSize      int
	batchDuration  time.Duration
	maxConcurrency int
	l              logger.LoggerV1
	localRetries   int
	retryBackoff   time.Duration
	shutdownWait   time.Duration
	// 以下为可选增强字段（预留）：单批处理超时、定时器抖动窗口、批次并发限流等。
	perFlushTimeout   time.Duration
	timerJitter       time.Duration
	timerJitterFactor float64 // 定时器抖动因子，减轻多 worker 同时触发 timer 的惊群。
	sem               chan struct{}
}

// NewAsyncBatchHandler 全参数构造函数。
func NewAsyncBatchHandler[T any](
	fn func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error,
	batchSize int,
	batchDuration time.Duration,
	maxConcurrency int,
	l logger.LoggerV1,
	localRetries int,
	retryBackoff time.Duration,
	shutdownWait time.Duration,
) *AsyncBatchHandler[T] {
	if batchSize <= 0 {
		batchSize = 10
	}
	if batchDuration <= 0 {
		batchDuration = time.Second
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	if retryBackoff <= 0 {
		retryBackoff = 100 * time.Millisecond
	}
	if localRetries <= 0 {
		localRetries = 3
	}
	if shutdownWait <= 0 {
		shutdownWait = 1 * time.Second
	}
	return &AsyncBatchHandler[T]{
		fn:                fn,
		batchSize:         batchSize,
		batchDuration:     batchDuration,
		maxConcurrency:    maxConcurrency,
		l:                 l,
		localRetries:      localRetries,
		retryBackoff:      retryBackoff,
		shutdownWait:      shutdownWait,
		timerJitterFactor: DEFAULT_JITTER_FACTOR,
	}
}

// NewAsyncBatchHandlerSimple 简化构造：只传 logger、fn、batchSize，其余使用库内默认值。
func NewAsyncBatchHandlerSimple[T any](
	l logger.LoggerV1,
	fn func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error,
	batchSize int,
) *AsyncBatchHandler[T] {
	return NewAsyncBatchHandler[T](
		fn,
		batchSize,
		time.Second,
		8,
		l,
		3,
		100*time.Millisecond,
		1*time.Second,
	)
}

func (b AsyncBatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b AsyncBatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 实现 sarama.ConsumerGroupHandler：主循环从 claim 收消息写入 taskCh，worker 聚批后调 fn，成功则经 ackCh 回写 MarkMessage。
func (b *AsyncBatchHandler[T]) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	ctx := session.Context()

	bufCap := b.maxConcurrency * b.batchSize
	if bufCap <= 0 {
		bufCap = 1
	}
	taskCh := make(chan task, bufCap)
	ackCh := make(chan *sarama.ConsumerMessage, bufCap)

	var wg sync.WaitGroup
	for i := 0; i < b.maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.worker(ctx, taskCh, ackCh)
		}()
	}

	msgsCh := claim.Messages()

	for {
		select {
		case <-ctx.Done():
			// Rebalance 或会话结束：尽快退出；先非阻塞 drain 已有 ack，不等待 worker 刷完当前批次。
			if b.l != nil {
				b.l.Info("消费会话取消，快速退出（未等待 worker 完成当前批次）")
			}
			close(taskCh)
		drainOnCancel:
			for {
				select {
				case ack := <-ackCh:
					session.MarkMessage(ack, "")
				default:
					break drainOnCancel
				}
			}
			return nil

		case ack := <-ackCh:
			session.MarkMessage(ack, "")

		case msg, ok := <-msgsCh:
			if !ok {
				close(taskCh)
				wg.Wait()
			DrainLoop2:
				for {
					select {
					case ack := <-ackCh:
						session.MarkMessage(ack, "")
					default:
						break DrainLoop2
					}
				}
				return nil
			}
			taskCh <- task{msg: msg}
		}
	}
}

// worker 从 taskCh 读单条消息，在内部做定时 / 满批聚合后调用业务 fn（带 ctx，可取消）。
func (b *AsyncBatchHandler[T]) worker(
	ctx context.Context,
	taskCh <-chan task,
	ackCh chan<- *sarama.ConsumerMessage,
) {
	var (
		msgsBuf []*sarama.ConsumerMessage = make([]*sarama.ConsumerMessage, 0, b.batchSize)
		tsBuf   []T                       = make([]T, 0, b.batchSize)
		timer   = time.NewTimer(calculateJitter(b.batchDuration, b.timerJitterFactor))
	)
	defer timer.Stop()

	flush := func() {
		if len(msgsBuf) == 0 {
			return
		}

		select {
		case <-ctx.Done():
			if b.l != nil {
				b.l.Warn("ctx canceled before flush start, skip batch")
			}
			return
		default:
		}

		attempt := 0
		for {
			err := b.fn(ctx, msgsBuf, tsBuf)

			if err == nil {
				for _, m := range msgsBuf {
					select {
					case ackCh <- m:
					default:
						// ackCh 满时阻塞写，保证最终能提交；也可在此打 metric。
						ackCh <- m
					}
				}
				break
			}

			attempt++
			if b.l != nil {
				b.l.Warn("batch fn failed", logger.Error(err), logger.Int("attempt", attempt))
			}
			if attempt > b.localRetries {
				if b.l != nil {
					b.l.Error("batch retries exhausted, ack & skip", logger.Error(err), logger.Int("attempt", attempt))
				}
				for _, m := range msgsBuf {
					select {
					case ackCh <- m:
					default:
						ackCh <- m
					}
				}
				break
			}

			base := b.retryBackoff * time.Duration(1<<uint(attempt-1))
			var d time.Duration
			if base > 0 {
				j := time.Duration(rand.Int63n(int64(base))) - base/2
				d = base + j
				if d < 0 {
					d = base
				}
			} else {
				d = b.retryBackoff
			}

			select {
			case <-ctx.Done():
				if b.l != nil {
					b.l.Warn("ctx done during backoff, aborting flush")
				}
				return
			case <-time.After(d):
			}
		}

		msgsBuf = msgsBuf[:0]
		tsBuf = tsBuf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			flush()
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
			timer.Reset(jitter)

		case task, ok := <-taskCh:
			if !ok {
				flush()
				return
			}
			msg := task.msg
			var t T
			if err := json.Unmarshal(msg.Value, &t); err != nil {
				if b.l != nil {
					b.l.Error("反序列化消息体失败", logger.Error(err),
						logger.String("topic", msg.Topic),
						logger.Int32("partition", msg.Partition),
						logger.Int64("offset", msg.Offset))
				}
				ackCh <- msg
				continue
			}
			msgsBuf = append(msgsBuf, msg)
			tsBuf = append(tsBuf, t)
			if len(msgsBuf) >= b.batchSize {
				flush()
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
				timer.Reset(jitter)
			}
		}
	}
}
