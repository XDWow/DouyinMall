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
	// 榛樿鎶栧姩鍥犲瓙锛岀敤浜庨槻姝㈡儕缇ゆ晥搴?
	// 鎶栧姩鑼冨洿锛歜ase 卤 (base * DEFAULT_JITTER_FACTOR)
	// 渚嬪锛?绉?卤 10% = [0.9s, 1.1s]
	DEFAULT_JITTER_FACTOR = 0.1
)

// 鍖呬竴灞傜殑鐩殑鏄负浜嗘墿灞曟€э紝鏇村鏄撴墿灞曟湭鏉ュ瓧娈碉紙濡傞噸璇曟鏁般€佹椂闂存埑绛夛級
type task struct{ msg *sarama.ConsumerMessage }

// calculateJitter 璁＄畻甯︽姈鍔ㄧ殑鎸佺画鏃堕棿
// 浣跨敤鏍囧噯鎶栧姩绠楁硶锛歜ase 卤 (base * jitterFactor)
// jitterFactor 寤鸿鑼冨洿锛?.05-0.2 (5%-20%)
func calculateJitter(base time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return base
	}
	// 鐢熸垚 [-jitterFactor, +jitterFactor] 鑼冨洿鍐呯殑闅忔満鏁?
	jitter := (rand.Float64()*2 - 1) * jitterFactor
	// 璁＄畻鏈€缁堟椂闂达細base * (1 + jitter)
	// 渚嬪锛歫itterFactor=0.1, jitter鑼冨洿[-0.1, +0.1], 鏈€缁堣寖鍥碵0.9, 1.1]
	return time.Duration(float64(base) * (1.0 + jitter))
}

// AsyncBatchHandler 娉涘瀷鎵归噺娑堣垂澶勭悊鍣?
// T 涓轰笟鍔℃暟鎹被鍨?
// fn: 鎵归噺澶勭悊鍑芥暟锛屾帴鏀?context 骞跺鐞嗕竴鎵规秷鎭?
// batchSize: 鍗曟鎵归噺澶у皬
// batchDuration: 鎵归噺瓒呮椂鏃堕棿锛岀‘淇濅笉浼氭棤闄愮瓑寰?
// maxConcurrency: 骞跺彂 worker 鏁伴噺
// l: logger
// localRetries: 鍗曚釜鎵规鏈湴蹇€熼噸璇曟鏁?
// retryBackoff: 閲嶈瘯鍩虹嚎锛堢敤浜庢寚鏁伴€€閬匡級
// shutdownWait: 鍦?session 鍏抽棴/rebalance 鏃讹紝涓诲崗绋嬬粰 worker/drain ack 鐨勫闄愭椂闂?
type AsyncBatchHandler[T any] struct {
	fn             func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error
	batchSize      int
	batchDuration  time.Duration
	maxConcurrency int
	l              logger.LoggerV1
	localRetries   int
	retryBackoff   time.Duration
	shutdownWait   time.Duration
	// 鍙€夊寮洪厤缃細鍗曟鎵瑰鐞嗚秴鏃躲€佸畾鏃跺櫒鎶栧姩绐楀彛銆佹壒娆″苟鍙戦檺娴?
	perFlushTimeout   time.Duration
	timerJitter       time.Duration
	timerJitterFactor float64 // 瀹氭椂鍣ㄦ姈鍔ㄥ洜瀛愶紝鐢ㄤ簬闃叉鎯婄兢鏁堝簲
	sem               chan struct{}
}

// NewAsyncBatchHandler 鏋勯€犲嚱鏁帮紙甯﹂粯璁ゅ€硷級
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
		timerJitterFactor: DEFAULT_JITTER_FACTOR, // 璁剧疆榛樿鎶栧姩鍥犲瓙
	}
}

// NewAsyncBatchHandlerSimple 绠€鍖栫増鏋勯€犲嚱鏁帮紝鍙帴鍙楀繀瑕佸弬鏁帮紝鍏朵粬浣跨敤榛樿鍊?
func NewAsyncBatchHandlerSimple[T any](
	l logger.LoggerV1,
	fn func(ctx context.Context, msgs []*sarama.ConsumerMessage, ts []T) error,
	batchSize int,
) *AsyncBatchHandler[T] {
	return NewAsyncBatchHandler[T](
		fn,                   // 澶勭悊鍑芥暟
		batchSize,            // 鎵规澶у皬
		time.Second,          // 鎵规鏃堕棿锛堥粯璁?绉掞級
		8,                    // 鏈€澶у苟鍙戞暟锛堥粯璁?锛?
		l,                    // 鏃ュ織鍣?
		3,                    // 鏈湴閲嶈瘯娆℃暟锛堥粯璁?锛?
		100*time.Millisecond, // 閲嶈瘯閫€閬挎椂闂达紙榛樿100ms锛?
		1*time.Second,        // 鍏抽棴绛夊緟鏃堕棿锛堥粯璁?绉掞級
	)
}

func (b AsyncBatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b AsyncBatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 瀹炵幇 sarama.ConsumerGroupHandler 鎺ュ彛
func (b *AsyncBatchHandler[T]) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	ctx := session.Context()

	// 甯︾紦鍐茬殑閫氶亾锛屽噺灏戠煭鏃堕樆濉?
	bufCap := b.maxConcurrency * b.batchSize
	if bufCap <= 0 {
		bufCap = 1
	}
	taskCh := make(chan task, bufCap)
	ackCh := make(chan *sarama.ConsumerMessage, bufCap)

	var wg sync.WaitGroup
	// 鍚姩鍥哄畾鏁伴噺鐨?worker
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
			// Rebalance 鎴栬€呬細璇濈粨鏉燂紝蹇€熼€€鍑?
			// 1) 绔嬪嵆鍋滄鎺ユ敹鏂颁换鍔?
			close(taskCh)

			// 2) 蹇€?drain 宸叉湁鐨?ack锛屼笉绛夊緟 worker
			// 杩欐牱 Sarama 鑳藉揩閫熷搷搴旓紝閬垮厤瓒呮椂
			for {
				select {
				case ack := <-ackCh:
					session.MarkMessage(ack, "")
				default:
				}
			}
			// 3) 涓荤嚎绋嬬洿鎺ラ€€鍑猴紝涓嶇瓑寰?worker 瀹屾垚褰撳墠鎵规
			// 姝ｅ湪鎵ц鐨?worker 浼氱户缁繍琛岋紝浣嗕富绾跨▼涓嶇瓑瀹?
			// 璁℃暟涓氬姟鍏佽灏戦噺鍋忓樊锛岄噸澶嶆秷璐瑰嚑娆″奖鍝嶅緢灏?
			b.l.Info("蹇€熼€€鍑猴紝涓嶇瓑寰?worker 瀹屾垚褰撳墠鎵规")
			return nil

		case ack := <-ackCh:
			// 鎸佺画鎻愪氦宸叉垚鍔熷鐞嗙殑娑堟伅
			session.MarkMessage(ack, "")

		case msg, ok := <-msgsCh:
			if !ok {
				// claim 鐨勬秷鎭€氶亾琚叧闂紝姝ｅ父閫€鍑猴細鍏堝叧闂?taskCh锛岀瓑寰呭苟 drain
				close(taskCh)
				// 绛夊緟 worker 閫€鍑哄苟 drain ackCh (绠€鐭?
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
			// 鍙戦€佺粰 worker
			taskCh <- task{msg: msg}
		}
	}
}

// worker 鎺ユ敹鍗曟潯娑堟伅锛屽唴閮ㄥ仛鎵归噺鑱氬悎骞惰皟鐢ㄤ笟鍔″嚱鏁?
// 浣跨敤甯?context 鐨?b.fn锛屾敮鎸佹湁鐣岀瓑寰呬笌鍙栨秷
func (b *AsyncBatchHandler[T]) worker(
	ctx context.Context,
	taskCh <-chan task,
	ackCh chan<- *sarama.ConsumerMessage,
) {
	var (
		msgsBuf []*sarama.ConsumerMessage = make([]*sarama.ConsumerMessage, 0, b.batchSize)
		tsBuf   []T                       = make([]T, 0, b.batchSize)
		// 鍒濆鍖?timer 鏃跺氨浣跨敤甯︽姈鍔ㄧ殑鏃堕棿锛岄槻姝㈠涓?worker 鍚屾椂瑙﹀彂
		timer = time.NewTimer(calculateJitter(b.batchDuration, b.timerJitterFactor))
	)
	defer timer.Stop()

	flush := func() {
		if len(msgsBuf) == 0 {
			return
		}

		// 鍦ㄥ紑濮嬪鐞嗗墠锛屽鏋?session 宸插彇娑堝垯涓嶅啀鍙戣捣鏂扮殑 b.fn
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
				// 鎴愬姛鍚庣粺涓€ ack
				for _, m := range msgsBuf {
					select {
					case ackCh <- m:
					default:
						// 濡傛灉 ackCh 婊′簡锛屽氨鍋氶樆濉炲紡鍐欎互淇濊瘉鏈€缁堣兘琚彁浜わ紱涔熷彲浠ヨ褰?metric
						ackCh <- m
					}
				}
				break
			}

			// 澶辫触锛氳褰曞苟鍐冲畾鏄惁閲嶈瘯
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

			// 璁＄畻鎸囨暟閫€閬?+ jitter
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

			// 鍦ㄩ€€閬挎湡闂村皧閲?ctx.Done()
			select {
			case <-ctx.Done():
				b.l.Warn("ctx done during backoff, aborting flush")
				return
			case <-time.After(d):
				// 缁х画涓嬩竴杞噸璇?
			}
		}

		// 閲嶇疆缂撳啿
		msgsBuf = msgsBuf[:0]
		tsBuf = tsBuf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// session 鍙栨秷鏃讹紝涓嶅彂璧锋柊鐨勫伐浣滐紱ConsumeClaim 浼?close(taskCh) 骞堕噰鍙?grace
			return

		case <-timer.C:
			// 瓒呮椂瑙﹀彂鎵归噺
			flush()
			// Safe reset with jitter to prevent thundering herd
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			// 浣跨敤鏍囧噯鎶栧姩绠楁硶闃叉鎯婄兢鏁堝簲
			jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
			timer.Reset(jitter)

		case task, ok := <-taskCh:
			if !ok {
				// 閫氶亾鍏抽棴锛屾彁浜ゅ墿浣欏悗閫€鍑?
				flush()
				return
			}
			msg := task.msg
			// 鍙嶅簭鍒楀寲
			var t T
			err := json.Unmarshal(msg.Value, &t)
			if err != nil {
				b.l.Error("鍙嶅簭鍒楀寲澶辫触", logger.Error(err),
					logger.String("topic", msg.Topic),
					logger.Int32("partition", msg.Partition),
					logger.Int64("offset", msg.Offset))
				// 璺宠繃骞剁洿鎺?ack锛岄伩鍏嶉樆濉炲悗缁?
				ackCh <- msg
			}
			// 绱Н
			msgsBuf = append(msgsBuf, msg)
			tsBuf = append(tsBuf, t)
			// 鍒拌揪鎵归噺澶у皬绔嬪嵆瑙﹀彂
			if len(msgsBuf) >= b.batchSize {
				flush()
				// 閲嶇疆瀹氭椂鍣?with jitter
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				// 浣跨敤鏍囧噯鎶栧姩绠楁硶闃叉鎯婄兢鏁堝簲
				jitter := calculateJitter(b.batchDuration, b.timerJitterFactor)
				timer.Reset(jitter)
			}
		}
	}
}


