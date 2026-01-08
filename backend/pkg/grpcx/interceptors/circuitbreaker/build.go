package circuitbreaker

import (
	"context"
	"github.com/go-kratos/aegis/circuitbreaker"
	"google.golang.org/grpc"
	"math/rand"
	"time"
)

type InterceptorBuilder struct {
	breaker circuitbreaker.CircuitBreaker

	// 考虑熔断恢复
	// 假如说我们考虑使用随机数 + 阈值的恢复方式
	// 触发熔断的时候，直接将 threshold 置为0
	// 后续等一段时间，将 theshold 调整为 1，判定请求有没有问题
	threshold int
}

func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if b.breaker.Allow() == nil {
			resp, err = handler(ctx, req)
			// 借助这个判定是不是业务错误
			//s, ok :=status.FromError(err)
			//if s != nil && s.Code() == codes.Unavailable {
			//	b.breaker.MarkFailed()
			//} else {
			//
			//}
			if err != nil {
				// 进一步区别是不是系统错
				// 我这边没有区别业务错误和系统错误
				b.breaker.MarkFailed()
			} else {
				b.breaker.MarkSuccess()
			}
		}
		b.breaker.MarkFailed()
		// 触发了熔断器
		return nil, err
	}
}

// 自定义熔断器
func (b *InterceptorBuilder) BuildServerInterceptorV1() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if !b.allow() {
			// 触发了熔断
			b.threshold = 0
			// 熔断恢复
			time.AfterFunc(time.Minute, func() {
				b.threshold = 1
			})
		}
		// 随机数判断，实现慢恢复
		rand := rand.Intn(100)
		if rand < b.threshold {
			resp, err = handler(ctx, req)
			if err == nil && b.threshold != 0 {
				// 你要考虑调大 threshold
				b.threshold++
			} else if b.threshold != 0 {
				// 你要考虑调小 threshold
				b.threshold--
			}
			return resp, err
		}
		return
	}
}

func (b *InterceptorBuilder) allow() bool {
	return false
}
