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
	// 默认抖动因子，用于防止惊群效应
	// 抖动范围：base ± (base * DEFAULT_JITTER_FACTOR)
	// 例如：1秒 ± 10% = [0.9s, 1.1s]
	DEFAULT_JITTER_FACTOR = 0.1
)

// 包一层的目的是为了扩展性，更容易扩展未来字段（如重试次数、时间戳等）
type task struct{ msg *sarama.ConsumerMessage }

// calculateJitter 计算带抖动的持续时间
// 使用标准抖动算法：base ± (base * jitterFactor)
// jitterFactor 建议范围：0.05-0.2 (5%-20%)
func calculateJitter(base time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return base
	}
	// 生成 [-jitterFactor, +jitterFactor] 范围内的随机数
	jitter := (rand.Float64()*2 - 1) * jitterFactor
	// 计算最终时间：base * (1 + jitter)
	// 例如：jitterFactor=0.1, jitter范围[-0.1, +0.1], 最终范围[0.9, 1.1]
	return time.Duration(float64(base) * (1.0 + jitter))
}

// AsyncBatchHandler 泛型批量消费处理器
// T 为业务数据类型
// fn: 批量处理函数，接收 context 并处理一批消息
// batchSize: 单次批量大小
// batchDuration: 批量超时时间，确保不会无限等待
// maxConcurrency: 并发 worker 数量
// l: logger
// localRetries: 单个批次本地快速重试次数
// retryBackoff: 重试基线（用于指数退避）
// shutdownWait: 在 session 关闭/rebalance 时，主协程给 worker/drain ack 的宽限时间
type AsyncBatchHandler[T any] struct {
	fn             func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error
	batchSize      int
	batchDuration  time.Duration
	maxConcurrency int
	l              logger.LoggerV1
	localRetries   int
	retryBackoff   time.Duration
	shutdownWait   time.Duration
	// 可选增强配置：单次批处理超时、定时器抖动窗口、批次并发限流
	perFlushTimeout   time.Duration
	timerJitter       time.Duration
	timerJitterFactor float64 // 定时器抖动因子，用于防止惊群效应
	sem               chan struct{}
}

// NewAsyncBatchHandler 构造函数（带默认值）
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
		timerJitterFactor: DEFAULT_JITTER_FACTOR, // 设置默认抖动因子
	}
}

// NewAsyncBatchHandlerSimple 简化版构造函数，只接受必要参数，其他使用默认值
func NewAsyncBatchHandlerSimple[T any](
	l logger.LoggerV1,
	fn func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error,
	batchSize int,
) *AsyncBatchHandler[T] {
	return NewAsyncBatchHandler[T](
		fn,                   // 处理函数
		batchSize,            // 批次大小
		time.Second,          // 批次时间（默认1秒）
		8,                    // 最大并发数（默认8）
		l,                    // 日志器
		3,                    // 本地重试次数（默认3）
		100*time.Millisecond, // 重试退避时间（默认100ms）
		1*time.Second,        // 关闭等待时间（默认1秒）
	)
}

func (b AsyncBatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b AsyncBatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 实现 sarama.ConsumerGroupHandler 接口
func (b *AsyncBatchHandler[T]) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	ctx := session.Context()

	// 带缓冲的通道，减少短时阻塞
	bufCap := b.maxConcurrency * b.batchSize
	if bufCap <= 0 {
		bufCap = 1
	}
	taskCh := make(chan task, bufCap)
	ackCh := make(chan *sarama.ConsumerMessage, bufCap)

	var wg sync.WaitGroup
	// 启动固定数量的 worker
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
			// Rebalance 或者会话结束，快速退出
			// 1) 立即停止接收新任务
			close(taskCh)

			// 2) 快速 drain 已有的 ack，不等待 worker
			// 这样 Sarama 能快速响应，避免超时
			for {
				select {
				case ack := <-ackCh:
					session.MarkMessage(ack, "")
				default:
				}
			}
			// 3) 主线程直接退出，不等待 worker 完成当前批次
			// 正在执行的 worker 会继续运行，但主线程不等它
			// 计数业务允许少量偏差，重复消费几次影响很小
			b.l.Info("快速退出，不等待 worker 完成当前批次")
			return nil

		case ack := <-ackCh:
			// 持续提交已成功处理的消息
			session.MarkMessage(ack, "")

		case msg, ok := <-msgsCh:
			if !ok {
				// claim 的消息通道被关闭，正常退出：先关闭 taskCh，等待并 drain
				close(taskCh)
				// 等待 worker 退出并 drain ackCh (简短)
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
			// 发送给 worker
			taskCh <- task{msg: msg}
		}
	}
}

// worker 接收单条消息，内部做批量聚合并调用业务函数
// 使用带 context 的 b.fn，支持有界等待与取消
func (b *AsyncBatchHandler[T]) worker(
	ctx context.Context,
	taskCh <-chan task,
	ackCh chan<- *sarama.ConsumerMessage,
) {
	var (
		msgsBuf []*sarama.ConsumerMessage = make([]*sarama.ConsumerMessage, 0, b.batchSize)
		tsBuf   []T                       = make([]T, 0, b.batchSize)
		// 初始化 timer 时就使用带抖动的时间，防止多个 worker 同时触发
		timer = time.NewTimer(calculateJitter(b.batchDuration, b.timerJitterFactor))
	)
	defer timer.Stop()

	flush := func() {
		if len(msgsBuf) == 0 {
			return
		}

		// 在开始处理前，如果 session 已取消则不再发起新的 b.fn
		select {
		case <-ctx.Done():
			b.l.Warn("ctx canceled before flush start, skip batch")
			return
		default:
		}

		attempt := 0
		for {
			err := b.fn(ctx, msgsBuf, tsBuf)

			if err == nil {
				// 成功后统一 ack
				for _, m := range msgsBuf {
					select {
					case ackCh <- m:
					default:
						// 如果 ackCh 满了，就做阻塞式写以保证最终能被提交；也可以记录 metric
						ackCh <- m
					}
				}
				break
			}

			// 失败：记录并决定是否重试
			attempt++
			b.l.Warn("batch fn failed", logger.Error(err), logger.Int("attempt", attempt))
			if attempt > b.localRetries {
				b.l.Error("batch retries exhausted, ack & skip", logger.Error(err), logger.Int("attempt", attempt))
				for _, m := range msgsBuf {
					select {
					case ackCh <- m:
					default:
						ackCh <- m
					}
				}
				break
			}

			// 计算指数退避 + jitter
			base := b.retryBackoff * time.Duration(1<<uint(attempt-1))
			// jitter: [base/2, base + base/2)
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

			// 在退避期间尊重 ctx.Done()
			select {
			case <-ctx.Done():
				b.l.Warn("ctx done during backoff, aborting flush")
				return
			case <-time.After(d):
				// 继续下一轮重试
			}
		}

		// 重置缓冲
		msgsBuf = msgsBuf[:0]
		tsBuf = tsBuf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// session 取消时，不发起新的工作；ConsumeClaim 会 close(taskCh) 并采取 grace
			return

		case <-timer.C:
			// 超时触发批量
			flush()
			// Safe reset with jitter to prevent thundering herd
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			// 使用标准抖动算法防止惊群效应
			jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
			timer.Reset(jitter)

		case task, ok := <-taskCh:
			if !ok {
				// 通道关闭，提交剩余后退出
				flush()
				return
			}
			msg := task.msg
			// 反序列化
			var t T
			err := json.Unmarshal(msg.Value, &t)
			if err != nil {
				b.l.Error("反序列化失败", logger.Error(err),
					logger.String("topic", msg.Topic),
					logger.Int32("partition", msg.Partition),
					logger.Int64("offset", msg.Offset))
				// 跳过并直接 ack，避免阻塞后续
				ackCh <- msg
			}
			// 累积
			msgsBuf = append(msgsBuf, msg)
			tsBuf = append(tsBuf, t)
			// 到达批量大小立即触发
			if len(msgsBuf) >= b.batchSize {
				flush()
				// 重置定时器 with jitter
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				// 使用标准抖动算法防止惊群效应
				jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
				timer.Reset(jitter)
			}
		}
	}
}
