package ai

import (
	"sync"
	"time"
)

// 自己写的一个轻量熔断器，熔断是为了防止一个坏节点拖慢整个系统

// 熔断器状态
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // 正常
	BreakerOpen                         // 熔断中
	BreakerHalfOpen                     // 试探中
)

// 轻量级熔断器（无第三方依赖）
// 状态机：Closed → Open（连续 N 次失败） → HalfOpen（cooldown 后试探） → Closed/Open
type CircuitBreaker struct {
	mu          sync.Mutex // 多 goroutine 访问熔断器状态这个共享资源，上锁
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
