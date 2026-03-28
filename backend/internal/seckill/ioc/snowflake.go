package ioc

import (
	"strconv"
	"sync/atomic"
	"time"
)

type SnowflakeIDGenerator struct{ seq atomic.Int64 }

func InitIDGenerator() *SnowflakeIDGenerator { return &SnowflakeIDGenerator{} }

func (g *SnowflakeIDGenerator) GenerateID() string {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	seq := g.seq.Add(1) % 1000
	return strconv.FormatInt(now*1000+seq, 10)
}
