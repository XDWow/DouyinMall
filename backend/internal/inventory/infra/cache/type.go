package cache

import (
	"context"
	"time"
)

type InventoryCache interface {
	// Lua 脚本执行（核心）
	// 用于：ReserveStock、ReleaseStock 的原子操作
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Hash 批量读取
	// 用于：RefundStock 批量读取预扣记录
	// 批量返回 []interface{} 与 fields 一一对应；不存在的 field 为 nil，可区分「0」与空
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// 数值自增
	// 用于：RefundStock 恢复 Redis 展示库存
	IncrBy(ctx context.Context, key string, delta int32) (int64, error)

	// 用于：CacheRepairJob 查询展示库存
	Get(ctx context.Context, key string) (string, error)

	// 用于：CacheRepairJob 修复展示库存
	Set(ctx context.Context, key string, value string, expiration time.Duration) (string, error)
}
