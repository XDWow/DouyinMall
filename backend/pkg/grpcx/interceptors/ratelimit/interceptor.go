package ratelimit

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InterceptorBuilder struct {
	limiter ratelimit.Limiter
	key     string
	l       logger.LoggerV1

	// key 鏄?FullMethod, value 鏄粯璁ゅ€肩殑 json
	//defaultValueMap map[string]string
}

// NewInterceptorBuilder key: user-service
// "limiter:service:user" 鏁翠釜搴旂敤銆侀泦缇ら檺娴?
// "limiter:service:user:UserService" user 閲岄潰鐨?UserService 闄愭祦
func NewInterceptorBuilder(limiter ratelimit.Limiter, key string, l logger.LoggerV1) *InterceptorBuilder {
	return &InterceptorBuilder{limiter: limiter, key: key, l: l}
}

func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			// err 涓嶄负nil锛屼綘瑕佽€冭檻浣犵敤淇濆畧鐨勶紝杩樻槸鐢ㄦ縺杩涚殑绛栫暐
			// 杩欐槸淇濆畧鐨勭瓥鐣?
			b.l.Error("鍒ゅ畾闄愭祦鍑虹幇闂", logger.Error(err))
			return nil, status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")

			// 杩欐槸婵€杩涚殑绛栫暐
			// return handler(ctx, req)
		}
		if limited {
			//defVal, ok := b.defaultValueMap[info.FullMethod]
			//if ok {
			//	err = json.Unmarshal([]byte(defVal), &resp)
			//	return defVal, err
			//}
			return nil, status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")
		}
		return handler(ctx, req)
	}
}

// 鐢ㄦ潵閰嶅悎鍚庨潰涓氬姟杩涜 闄嶇骇
func (b *InterceptorBuilder) BuildServerInterceptorV1() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		// 杩欓噷涔熷弽鏄狅細鐔旀柇銆侀檺娴併€侀檷绾т箣闂存病鏈夋槑鏄剧殑鐣岄檺
		// 瑙﹀彂闄愭祦涔嬪悗锛氬彲浠ョ啍鏂紝鍙互闄嶇骇
		if err != nil || limited {
			// 寰堥毦鍋氬嚭缁熶竴鐨勯檷绾х瓥鐣ワ紝鍥犱负鍏朵簬涓氬姟娣卞害宓屽悎锛屽彧闇€鏍囪闄嶇骇浜嗭紝鍚庨潰涓氬姟鍐嶅叿浣撴墽琛岄檷绾?
			ctx = context.WithValue(ctx, "limited", "true")
		}

		return handler(ctx, req)
	}
}

func (b *InterceptorBuilder) BuildClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			// err 涓嶄负nil锛屼綘瑕佽€冭檻浣犵敤淇濆畧鐨勶紝杩樻槸鐢ㄦ縺杩涚殑绛栫暐
			// 杩欐槸淇濆畧鐨勭瓥鐣?
			b.l.Error("鍒ゅ畾闄愭祦鍑虹幇闂", logger.Error(err))
			return status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")
			// 杩欐槸婵€杩涚殑绛栫暐
			// return handler(ctx, req)
		}
		if limited {
			return status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// 鏈嶅姟绾у埆闄愭祦
func (b *InterceptorBuilder) BuildServerInterceptorService() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// info.FullMethod 鏉ヨ嚜鐢熸垚鐨?grpc 浠ｇ爜
		// 澶栭潰 if 鐩存帴鍒ゆ柇浜嗭紝閬垮厤鍏朵粬鏂规硶杩樻墽琛?Limit:鍘?redis 閲岄潰鏌?
		if strings.HasPrefix(info.FullMethod, "/UserService") {
			limited, err := b.limiter.Limit(ctx, "limiter:service:user:UserService")
			if err != nil {
				// err 涓嶄负nil锛屼綘瑕佽€冭檻浣犵敤淇濆畧鐨勶紝杩樻槸鐢ㄦ縺杩涚殑绛栫暐
				// 杩欐槸淇濆畧鐨?
				b.l.Error("鍒ゅ畾闄愭祦鍑虹幇闂", logger.Error(err))
				return nil, status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")
				// 杩欐槸婵€杩涚殑绛栫暐
				// return handler(ctx, req)
			}
			if limited {
				return nil, status.Errorf(codes.ResourceExhausted, "瑙﹀彂闄愭祦")
			}
		}
		return handler(ctx, req)
	}
}


