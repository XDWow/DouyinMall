package ratelimit

import "context"

type Limiter interface {
	// bool 浠ｈ〃鏄惁闄愭祦锛宔rr 闄愭祦鍣ㄦ湰韬湁娌℃湁閿欒
	//key 鏄檺娴佸璞?
	Limit(ctx context.Context, key string) (bool, error)
}


