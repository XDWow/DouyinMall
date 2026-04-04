package ioc

import (
	"sync"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"github.com/spf13/viper"
)

// snowflakeGenerator 绠€鍗曢洩鑺盜D鐢熸垚鍣紙41浣嶆绉掓椂闂存埑 | 10浣嶈妭鐐?| 12浣嶅簭鍒楀彿锛?
type snowflakeGenerator struct {
	mu     sync.Mutex
	nodeID int64
	seq    int64
	lastMS int64
}

func (g *snowflakeGenerator) GenerateOrderID() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UnixMilli()
	if now == g.lastMS {
		g.seq = (g.seq + 1) & 0xFFF
		if g.seq == 0 {
			// 绛夊緟涓嬩竴姣
			for now <= g.lastMS {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		g.seq = 0
		g.lastMS = now
	}
	return (now&0x1FFFFFFFFFF)<<22 | (g.nodeID&0x3FF)<<12 | g.seq
}

// InitIDGenerator 鍒涘缓闆姳ID鐢熸垚鍣紝鑺傜偣ID浠庨厤缃鍙栵紙榛樿1锛?
func InitIDGenerator() usecase.IDGenerator {
	nodeID := viper.GetInt64("snowflake.node_id")
	if nodeID <= 0 {
		nodeID = 1
	}
	return &snowflakeGenerator{nodeID: nodeID}
}


