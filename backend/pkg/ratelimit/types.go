package ratelimit

import "context"

type Limiter interface {
	// bool 代表是否限流，err 限流器本身有没有错误
	//key 是限流对象
	Limit(ctx context.Context, key string) (bool, error)
}
