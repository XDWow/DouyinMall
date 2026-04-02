//go:build legacy_agent

package ai

import (
	"sync"
	"time"
)

// 鑷繁鍐欑殑涓€涓交閲忕啍鏂櫒锛岀啍鏂槸涓轰簡闃叉涓€涓潖鑺傜偣鎷栨參鏁翠釜绯荤粺

// 鐔旀柇鍣ㄧ姸鎬?
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // 姝ｅ父
	BreakerOpen                         // 鐔旀柇涓?
	BreakerHalfOpen                     // 璇曟帰涓?
)

// 杞婚噺绾х啍鏂櫒锛堟棤绗笁鏂逛緷璧栵級
// 鐘舵€佹満锛欳losed 鈫?Open锛堣繛缁?N 娆″け璐ワ級 鈫?HalfOpen锛坈ooldown 鍚庤瘯鎺級 鈫?Closed/Open
type CircuitBreaker struct {
	mu          sync.Mutex // 澶?goroutine 璁块棶鐔旀柇鍣ㄧ姸鎬佽繖涓叡浜祫婧愶紝涓婇攣
	state       BreakerState
	failures    int32
	threshold   int32
	lastFailure time.Time
	cooldown    time.Duration
}

func NewCircuitBreaker(threshold int32, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     BreakerClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case BreakerClosed:
		return true
	case BreakerOpen:
		if time.Since(cb.lastFailure) > cb.cooldown {
			cb.state = BreakerHalfOpen
			return true
		}
		return false
	case BreakerHalfOpen:
		return false
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = BreakerClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = BreakerOpen
	}
}

func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
