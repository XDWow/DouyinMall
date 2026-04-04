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

// 	瀹炵幇
//	balancer.Picker
// 	base.PickerBuilder
//	鎺ュ彛

func init() {
	// NewBalancerBuilder 鏄府鎴戜滑鎶婁竴涓?Picker Builder 杞寲涓轰竴涓?balancer.Builder
	balancer.Register(base.NewBalancerBuilder(name, &PickerBuilder{}, base.Config{HealthCheck: false}))
}

type Picker struct {
	//	 杩欎釜鎵嶆槸鐪熺殑鎵ц璐熻浇鍧囪　鐨勫湴鏂?
	conns []*conn
	mutex sync.Mutex
}

// Pick 鍦ㄨ繖閲屽疄鐜板熀浜庢潈閲嶇殑璐熻浇鍧囪　绠楁硶
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.conns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	var total int
	var maxCC *conn
	for _, cc := range p.conns {
		// 鎬ц兘鏈€濂藉氨鏄湪 cc 涓婄敤鍘熷瓙鎿嶄綔
		// 浣嗘槸绛涢€夌粨鏋滀笉浼氫弗鏍肩鍚?WRR 绠楁硶
		// 鏁翠綋鏁堟灉鍙互
		if !cc.available {
			continue
		}
		total += cc.weight
		cc.currentWeight = cc.weight + cc.currentWeight
		if maxCC == nil || cc.currentWeight > maxCC.currentWeight {
			maxCC = cc
		}
	}
	// 鏇存柊锛岃繑鍥?maxCC
	maxCC.currentWeight -= total
	return balancer.PickResult{
		SubConn: maxCC.cc,
		Done: func(info balancer.DoneInfo) {
			// 寰堝鍔ㄦ€佺畻娉曪紝鏍规嵁璋冪敤缁撴灉鏉ヨ皟鏁存潈閲嶏紝灏卞湪杩欓噷
			// 鍥犱负鍦ㄨ繖閲屽彲浠ユ嬁鍒扮粨鏋滐紝杩涜鐔旀柇銆侀檷绾с€侀檺娴佹搷浣滐紝
			//浠ュ強 failover:澶辫触浜嗗氨鏍囪涓嶅彲鐢紝涓嬫杞灏变笉浼氬埌瀹?
			err := info.Err
			if err == nil {
				return
			}
			switch err {
			// 涓€鑸槸涓诲姩鍙栨秷锛屼綘娌″繀瑕佸幓璋?
			case context.Canceled:
				return
			case io.EOF, io.ErrUnexpectedEOF:
				// 鍩烘湰鍙互璁や负杩欎釜鑺傜偣宸茬粡宕╀簡
				maxCC.available = true
			// 鐪嬭繑鍥炵殑 code锛岃繘琛屽鐞?
			default:
				st, ok := status.FromError(err)
				if ok {
					code := st.Code()
					switch code {
					case codes.Unavailable:
						maxCC.available = false
						go func() {
							// 浣犺寮€涓€涓澶栫殑 goroutine 鍘绘帰娲?
							// 鍊熷姪 health check
							// for 寰幆
							if p.healthCheck(maxCC) {
								maxCC.available = true
								// 鍒氭斁鍥炴潵瑕侀檺娴佷竴浼氾紝闃叉鎶栧姩
								// 鍙互淇敼 weight, currentWeight
								// 鎴栬€呬笅涓€娆￠€変腑璇ヨ妭鐐规椂锛屾幏楠板瓙
							}
						}()
					case codes.ResourceExhausted:
						// 鏈€濂芥槸 currentWeight 鍜?weight 閮借皟浣?
						// 鍑忓皯瀹冭閫変腑鐨勬鐜?

						// 鍔犱竴涓敊璇爜琛ㄨ揪闄嶇骇
					}
				}
			}
		},
	}, nil
}

type PickerBuilder struct {
}

func (p *PickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	// 鏋勯€?Picker, 鐪嬪叾缁撴瀯浣擄紝瀹為檯涓婃槸鏋勯€?conns []*conn
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
			// 鍙互缁欎釜榛樿鍊?
			cc.weight = 10
		}
		conns = append(conns, cc)
	}
	return &Picker{
		conns: conns,
	}
}

func (p *Picker) healthCheck(cc *conn) bool {
	// 璋冪敤 grpc 鍐呯疆鐨勯偅涓?health check 鎺ュ彛
	return true
}

// conn 浠ｈ〃涓€涓妭鐐?
type conn struct {
	//	鐪熸鐨?grpc 閲岄潰鐨勪唬琛ㄤ竴涓妭鐐圭殑琛ㄨ揪
	cc balancer.SubConn

	// 鐢ㄤ簬 wrr
	weight        int
	currentWeight int

	available bool

	// 鍋囧鏈?vip 鎴栬€呴潪 vip
	group string
}


