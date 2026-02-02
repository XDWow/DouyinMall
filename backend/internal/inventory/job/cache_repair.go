package job

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

/*
大厂级定时任务设计：

核心原则：
1. Redis预库存 = DB库存 - 正在预扣中的量（这是正常状态）
2. 定时任务要区分"正常预扣差异" vs "真实不一致"
3. 只修复真实不一致，不破坏正常预扣

修复策略：
- 增量修复（hourly）：检查最近操作过的商品，计算预扣量后对比
- 全量对账（daily/3AM）：扫描所有活跃商品，全量校验
- 分布式锁：防止多实例并发执行
- 分批处理：每批100个商品，避免内存爆炸
- 监控告警：差异超过阈值（>10个商品或>5%库存）触发P1
*/

// CacheRepairJob Redis缓存一致性修复定时任务
type CacheRepairJob struct {
	db          *gorm.DB
	cache       cache.InventoryCache
	redisClient redis.Cmdable // 需要原生client执行SCAN等命令
	logger      logger.LoggerV1
}

func NewCacheRepairJob(db *gorm.DB, cache cache.InventoryCache, redisClient redis.Cmdable, l logger.LoggerV1) *CacheRepairJob {
	return &CacheRepairJob{
		db:          db,
		cache:       cache,
		redisClient: redisClient,
		logger:      l,
	}
}

// RunHourly 每小时增量修复：只检查最近1小时有操作的商品
func (j *CacheRepairJob) RunHourly() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 分布式锁：防止多实例并发执行
	lockKey := "lock:cache_repair:hourly"
	locked, err := j.redisClient.SetNX(ctx, lockKey, "1", 10*time.Minute).Result()
	if err != nil || !locked {
		j.logger.Info("Redis缓存修复任务已在其他实例执行，跳过")
		return nil
	}
	defer j.redisClient.Del(ctx, lockKey)

	j.logger.Info("开始执行Redis缓存增量修复（hourly）")

	// 1. 查询最近1小时有操作的商品ID（去重）
	var productIDs []int64
	err = j.db.WithContext(ctx).
		Model(&db.InventoryOperation{}).
		Distinct("product_id").
		Where("created_at > ?", time.Now().Add(-1*time.Hour)).
		Pluck("product_id", &productIDs).Error
	if err != nil {
		j.logger.Error("查询活跃商品失败", logger.Error(err))
		return err
	}

	if len(productIDs) == 0 {
		j.logger.Info("无需修复：最近1小时无库存操作")
		return nil
	}

	j.logger.Info("开始增量修复", logger.Int("productCount", len(productIDs)))

	// 2. 分批检查修复（每批100个）
	repairCount := 0
	var repairErrors []string

	for i := 0; i < len(productIDs); i += 100 {
		end := i + 100
		if end > len(productIDs) {
			end = len(productIDs)
		}
		batch := productIDs[i:end]

		for _, productID := range batch {
			if err := j.repairProduct(ctx, productID); err != nil {
				repairErrors = append(repairErrors, fmt.Sprintf("productID:%d error:%v", productID, err))
			} else if err == nil {
				repairCount++ // repairProduct返回nil表示修复了
			}
		}
	}

	// 3. 记录结果
	if len(repairErrors) > 0 {
		j.logger.Warn("增量修复完成（有失败）",
			logger.Int("repairCount", repairCount),
			logger.Int("errorCount", len(repairErrors)))
	} else if repairCount > 0 {
		j.logger.Info("增量修复完成", logger.Int("repairCount", repairCount))
	} else {
		j.logger.Info("增量检查通过，无需修复")
	}

	return nil
}

// RunDaily 每日全量对账（凌晨3点执行）
func (j *CacheRepairJob) RunDaily() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// 分布式锁
	lockKey := "lock:cache_repair:daily"
	locked, err := j.redisClient.SetNX(ctx, lockKey, "1", 60*time.Minute).Result()
	if err != nil || !locked {
		j.logger.Info("每日对账任务已在其他实例执行，跳过")
		return nil
	}
	defer j.redisClient.Del(ctx, lockKey)

	j.logger.Info("开始执行每日全量Redis对账")

	// 1. 查询所有有操作记录的商品（最近30天）
	var productIDs []int64
	err = j.db.WithContext(ctx).
		Model(&db.InventoryOperation{}).
		Distinct("product_id").
		Where("created_at > ?", time.Now().AddDate(0, 0, -30)).
		Pluck("product_id", &productIDs).Error
	if err != nil {
		j.logger.Error("查询活跃商品失败", logger.Error(err))
		return err
	}

	j.logger.Info("开始全量对账", logger.Int("productCount", len(productIDs)))

	// 2. 分批处理（每批100个）
	repairCount := 0
	var repairErrors []string

	for i := 0; i < len(productIDs); i += 100 {
		end := i + 100
		if end > len(productIDs) {
			end = len(productIDs)
		}
		batch := productIDs[i:end]

		for _, productID := range batch {
			if err := j.repairProduct(ctx, productID); err != nil {
				repairErrors = append(repairErrors, fmt.Sprintf("productID:%d error:%v", productID, err))
			} else if err == nil {
				repairCount++
			}
		}
	}

	// 3. 结果统计
	if repairCount > 10 || float64(repairCount)/float64(len(productIDs)) > 0.05 {
		// 大量差异（>10个或>5%），可能系统性问题，触发P1告警
		repairRate := float64(repairCount) / float64(len(productIDs)) * 100
		j.logger.Error("每日对账发现大量差异，请人工介入",
			logger.Int("repairCount", repairCount),
			logger.Int("totalProducts", len(productIDs)),
			logger.String("repairRate", fmt.Sprintf("%.2f%%", repairRate)))
	} else if repairCount > 0 {
		j.logger.Warn("每日对账完成", logger.Int("repairCount", repairCount))
	} else {
		j.logger.Info("每日对账通过，Redis与DB一致")
	}

	if len(repairErrors) > 0 {
		j.logger.Warn("对账过程有失败", logger.Int("errorCount", len(repairErrors)))
	}

	return nil
}

// repairProduct 修复单个商品的Redis预库存
// 返回 nil 表示修复了，返回 error 表示失败，返回特殊值表示无需修复
func (j *CacheRepairJob) repairProduct(ctx context.Context, productID int64) error {
	// 1. 查询DB库存（真实库存）
	var inventory db.Inventory
	err := j.db.WithContext(ctx).Where("product_id = ?", productID).First(&inventory).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("skip") // 商品已删除，跳过
		}
		return fmt.Errorf("查询DB失败: %w", err)
	}

	// 2. 查询Redis预库存
	redisStockStr, err := j.cache.Get(ctx, fmt.Sprintf("stock:%d", productID))
	if err != nil {
		// Redis没数据很正常（可能从未预扣过），跳过
		return fmt.Errorf("skip")
	}
	redisStock, _ := strconv.ParseInt(redisStockStr, 10, 64)

	// 3. 计算正在预扣中的量（扫描所有 reserve:* keys）
	// 大厂优化：这里可以用Bloom Filter预过滤，避免SCAN所有keys
	reservedQty, err := j.calculateReservedQuantity(ctx, productID)
	if err != nil {
		return fmt.Errorf("计算预扣量失败: %w", err)
	}

	// 4. 计算期望的Redis预库存
	expectedRedisStock := inventory.Stock - reservedQty

	// 5. 对比差异
	diff := abs64(redisStock - expectedRedisStock)
	threshold := max64(int64(float64(inventory.Stock)*0.05), 10) // 5%或10，取大值

	if diff <= threshold {
		// 差异在合理范围内，无需修复
		return fmt.Errorf("skip")
	}

	// 6. 差异超过阈值，修复Redis
	_, err = j.cache.Set(ctx, fmt.Sprintf("stock:%d", productID), strconv.FormatInt(expectedRedisStock, 10), 30*time.Minute)
	if err != nil {
		return fmt.Errorf("修复Redis失败: %w", err)
	}

	j.logger.Warn("检测到Redis预库存差异，已修复",
		logger.Int64("productID", productID),
		logger.Int64("dbStock", inventory.Stock),
		logger.Int64("reservedQty", reservedQty),
		logger.Int64("expectedRedis", expectedRedisStock),
		logger.Int64("actualRedis", redisStock),
		logger.Int64("diff", diff))

	return nil // 返回nil表示修复成功
}

// calculateReservedQuantity 计算商品的预扣总量
// 扫描所有 reserve:* keys，累加该商品的预扣数量
func (j *CacheRepairJob) calculateReservedQuantity(ctx context.Context, productID int64) (int64, error) {
	var totalReserved int64
	productIDStr := strconv.FormatInt(productID, 10)

	// 使用SCAN命令遍历所有预扣记录（大厂优化：可以维护一个set存储活跃的reserveID）
	var cursor uint64
	for {
		// 每次扫描100个key
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "reserve:*", 100).Result()
		if err != nil {
			return 0, fmt.Errorf("SCAN失败: %w", err)
		}

		// 批量查询这些预扣记录中该商品的数量
		for _, key := range keys {
			qtyStr, err := j.redisClient.HGet(ctx, key, productIDStr).Result()
			if err != nil {
				if err == redis.Nil {
					continue // 该预扣记录不包含此商品
				}
				j.logger.Warn("查询预扣记录失败", logger.String("key", key), logger.Error(err))
				continue
			}

			qty, _ := strconv.ParseInt(qtyStr, 10, 64)
			totalReserved += abs64(qty) // 预扣记录存的是负数，取绝对值
		}

		cursor = nextCursor
		if cursor == 0 {
			break // 扫描完成
		}
	}

	return totalReserved, nil
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
