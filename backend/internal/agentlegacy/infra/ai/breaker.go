//go:build legacy_agent

package ai

import (
	"sync"
	"time"
)

// 閼奉亜绻侀崘娆戞畱娑撯偓娑擃亣浜ら柌蹇曞晬閺傤厼娅掗敍宀€鍟嶉弬顓熸Ц娑撹桨绨￠梼鍙夘剾娑撯偓娑擃亜娼栭懞鍌滃仯閹锋牗鍙冮弫缈犻嚋缁崵绮?
// 閻旀梹鏌囬崳銊уЦ閹?
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // 濮濓絽鐖?	BreakerOpen                         // 閻旀梹鏌囨稉?
	BreakerHalfOpen                     // 鐠囨洘甯版稉?
)

// 鏉炲鍣虹痪褏鍟嶉弬顓炴珤閿涘牊妫ょ粭顑跨瑏閺傞€涚贩鐠ф牭绱?// 閻樿埖鈧焦婧€閿涙losed 閳?Open閿涘牐绻涚紒?N 濞嗏€炽亼鐠愩儻绱?閳?HalfOpen閿涘潏ooldown 閸氬氦鐦幒顫礆 閳?Closed/Open
type CircuitBreaker struct {
	mu          sync.Mutex // 婢?goroutine 鐠佸潡妫堕悢鏃€鏌囬崳銊уЦ閹浇绻栨稉顏勫彙娴滎偉绁┃鎰剁礉娑撳﹪鏀?	state       BreakerState
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


