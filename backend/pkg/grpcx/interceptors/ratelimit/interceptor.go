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

	// key 是 FullMethod, value 是默认值的 json
	//defaultValueMap map[string]string
}

// NewInterceptorBuilder key: user-service
// "limiter:service:user" 整个应用、集群限流
// "limiter:service:user:UserService" user 里面的 UserService 限流
func NewInterceptorBuilder(limiter ratelimit.Limiter, key string, l logger.LoggerV1) *InterceptorBuilder {
	return &InterceptorBuilder{limiter: limiter, key: key, l: l}
}

func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			// err 不为nil，你要考虑你用保守的，还是用激进的策略
			// 这是保守的策略
			b.l.Error("判定限流出现问题", logger.Error(err))
			return nil, status.Errorf(codes.ResourceExhausted, "触发限流")

			// 这是激进的策略
			// return handler(ctx, req)
		}
		if limited {
			//defVal, ok := b.defaultValueMap[info.FullMethod]
			//if ok {
			//	err = json.Unmarshal([]byte(defVal), &resp)
			//	return defVal, err
			//}
			return nil, status.Errorf(codes.ResourceExhausted, "触发限流")
		}
		return handler(ctx, req)
	}
}

// 用来配合后面业务进行 降级
func (b *InterceptorBuilder) BuildServerInterceptorV1() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		// 这里也反映：熔断、限流、降级之间没有明显的界限
		// 触发限流之后：可以熔断，可以降级
		if err != nil || limited {
			// 很难做出统一的降级策略，因为其于业务深度嵌合，只需标记降级了，后面业务再具体执行降级
			ctx = context.WithValue(ctx, "limited", "true")
		}

		return handler(ctx, req)
	}
}

func (b *InterceptorBuilder) BuildClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			// err 不为nil，你要考虑你用保守的，还是用激进的策略
			// 这是保守的策略
			b.l.Error("判定限流出现问题", logger.Error(err))
			return status.Errorf(codes.ResourceExhausted, "触发限流")
			// 这是激进的策略
			// return handler(ctx, req)
		}
		if limited {
			return status.Errorf(codes.ResourceExhausted, "触发限流")
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// 服务级别限流
func (b *InterceptorBuilder) BuildServerInterceptorService() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// info.FullMethod 来自生成的 grpc 代码
		// 外面 if 直接判断了，避免其他方法还执行 Limit:去 redis 里面查
		if strings.HasPrefix(info.FullMethod, "/UserService") {
			limited, err := b.limiter.Limit(ctx, "limiter:service:user:UserService")
			if err != nil {
				// err 不为nil，你要考虑你用保守的，还是用激进的策略
				// 这是保守的
				b.l.Error("判定限流出现问题", logger.Error(err))
				return nil, status.Errorf(codes.ResourceExhausted, "触发限流")
				// 这是激进的策略
				// return handler(ctx, req)
			}
			if limited {
				return nil, status.Errorf(codes.ResourceExhausted, "触发限流")
			}
		}
		return handler(ctx, req)
	}
}
