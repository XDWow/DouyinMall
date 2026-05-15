package ioc

import (
	"sync"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"github.com/spf13/viper"
)

// snowflakeGenerator is a simple Snowflake-style ID generator:
// 41 bits timestamp | 10 bits node ID | 12 bits sequence.
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
			// Wait for the next millisecond when the local sequence overflows.
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

// InitIDGenerator builds the order ID generator from config.
func InitIDGenerator() usecase.IDGenerator {
	nodeID := viper.GetInt64("snowflake.node_id")
	if nodeID <= 0 {
		nodeID = 1
	}
	return &snowflakeGenerator{nodeID: nodeID}
}
