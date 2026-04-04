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

	// 鑰冭檻鐔旀柇鎭㈠
	// 鍋囧璇存垜浠€冭檻浣跨敤闅忔満鏁?+ 闃堝€肩殑鎭㈠鏂瑰紡
	// 瑙﹀彂鐔旀柇鐨勬椂鍊欙紝鐩存帴灏?threshold 缃负0
	// 鍚庣画绛変竴娈垫椂闂达紝灏?theshold 璋冩暣涓?1锛屽垽瀹氳姹傛湁娌℃湁闂
	threshold int
}

func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if b.breaker.Allow() == nil {
			resp, err = handler(ctx, req)
			// 鍊熷姪杩欎釜鍒ゅ畾鏄笉鏄笟鍔￠敊璇?
			//s, ok :=status.FromError(err)
			//if s != nil && s.Code() == codes.Unavailable {
			//	b.breaker.MarkFailed()
			//} else {
			//
			//}
			if err != nil {
				// 杩涗竴姝ュ尯鍒槸涓嶆槸绯荤粺閿?
				// 鎴戣繖杈规病鏈夊尯鍒笟鍔￠敊璇拰绯荤粺閿欒
				b.breaker.MarkFailed()
			} else {
				b.breaker.MarkSuccess()
			}
		}
		b.breaker.MarkFailed()
		// 瑙﹀彂浜嗙啍鏂櫒
		return nil, err
	}
}

// 鑷畾涔夌啍鏂櫒
func (b *InterceptorBuilder) BuildServerInterceptorV1() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if !b.allow() {
			// 瑙﹀彂浜嗙啍鏂?
			b.threshold = 0
			// 鐔旀柇鎭㈠
			time.AfterFunc(time.Minute, func() {
				b.threshold = 1
			})
		}
		// 闅忔満鏁板垽鏂紝瀹炵幇鎱㈡仮澶?
		rand := rand.Intn(100)
		if rand < b.threshold {
			resp, err = handler(ctx, req)
			if err == nil && b.threshold != 0 {
				// 浣犺鑰冭檻璋冨ぇ threshold
				b.threshold++
			} else if b.threshold != 0 {
				// 浣犺鑰冭檻璋冨皬 threshold
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


