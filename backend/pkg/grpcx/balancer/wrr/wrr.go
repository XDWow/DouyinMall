package wrr

import (
	"context"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"sync"
)

const name = "custom_wrr"

// 	实现
//	balancer.Picker
// 	base.PickerBuilder
//	接口

func init() {
	// NewBalancerBuilder 是帮我们把一个 Picker Builder 转化为一个 balancer.Builder
	balancer.Register(base.NewBalancerBuilder(name, &PickerBuilder{}, base.Config{HealthCheck: false}))
}

type Picker struct {
	//	 这个才是真的执行负载均衡的地方
	conns []*conn
	mutex sync.Mutex
}

// Pick 在这里实现基于权重的负载均衡算法
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.conns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	var total int
	var maxCC *conn
	for _, cc := range p.conns {
		// 性能最好就是在 cc 上用原子操作
		// 但是筛选结果不会严格符合 WRR 算法
		// 整体效果可以
		if !cc.available {
			continue
		}
		total += cc.weight
		cc.currentWeight = cc.weight + cc.currentWeight
		if maxCC == nil || cc.currentWeight > maxCC.currentWeight {
			maxCC = cc
		}
	}
	// 更新，返回 maxCC
	maxCC.currentWeight -= total
	return balancer.PickResult{
		SubConn: maxCC.cc,
		Done: func(info balancer.DoneInfo) {
			// 很多动态算法，根据调用结果来调整权重，就在这里
			// 因为在这里可以拿到结果，进行熔断、降级、限流操作，
			//以及 failover:失败了就标记不可用，下次轮询就不会到它
			err := info.Err
			if err == nil {
				return
			}
			switch err {
			// 一般是主动取消，你没必要去调
			case context.Canceled:
				return
			case io.EOF, io.ErrUnexpectedEOF:
				// 基本可以认为这个节点已经崩了
				maxCC.available = true
			// 看返回的 code，进行处理
			default:
				st, ok := status.FromError(err)
				if ok {
					code := st.Code()
					switch code {
					case codes.Unavailable:
						maxCC.available = false
						go func() {
							// 你要开一个额外的 goroutine 去探活
							// 借助 health check
							// for 循环
							if p.healthCheck(maxCC) {
								maxCC.available = true
								// 刚放回来要限流一会，防止抖动
								// 可以修改 weight, currentWeight
								// 或者下一次选中该节点时，掷骰子
							}
						}()
					case codes.ResourceExhausted:
						// 最好是 currentWeight 和 weight 都调低
						// 减少它被选中的概率

						// 加一个错误码表达降级
					}
				}
			}
		},
	}, nil
}

type PickerBuilder struct {
}

func (p *PickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	// 构造 Picker, 看其结构体，实际上是构造 conns []*conn
	conns := make([]*conn, 0, len(info.ReadySCs))
	// sc => SubConn
	// sci => SubConnInfo
	for sc, sci := range info.ReadySCs {
		cc := &conn{
			cc: sc,
		}
		md, ok := sci.Address.Metadata.(map[string]any)
		if ok {
			weigthVal := md["weight"]
			weight, _ := weigthVal.(float64)
			cc.weight = int(weight)
		}
		if cc.weight == 0 {
			// 可以给个默认值
			cc.weight = 10
		}
		conns = append(conns, cc)
	}
	return &Picker{
		conns: conns,
	}
}

func (p *Picker) healthCheck(cc *conn) bool {
	// 调用 grpc 内置的那个 health check 接口
	return true
}

// conn 代表一个节点
type conn struct {
	//	真正的 grpc 里面的代表一个节点的表达
	cc balancer.SubConn

	// 用于 wrr
	weight        int
	currentWeight int

	available bool

	// 假如有 vip 或者非 vip
	group string
}
