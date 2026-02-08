package job

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/repository"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

/*
	兜底搞不了，库存兜底不是在线修复问题，而是事故处理流程
	通过冻结库存写入，强制推进订单到终态，再以 DB 为唯一事实源重建redis数据
*/

/*
定时任务设计：

核心原则：
1. Redis预库存 = DB库存 - 正在预扣中的量
2. 扫描 Redis 中有库存缓存的商品，检查是否正确

对账策略：
- 频率：每天凌晨3点
- 范围：扫描 Redis 所有 stock:* keys（有缓存的商品）
- 分布式锁：防止多实例并发执行
- 分批处理：SCAN 每次100个key，避免阻塞 Redis
- 监控告警：修复商品数 > 5% 触发告警

为什么扫描 Redis 而不是查 DB 操作表：
- DB 操作表只记录 CommitStock/RefundStock
- ReserveStock/ReleaseStock 不写 DB，只操作 Redis
- 所以预扣/释放失败导致的不一致，查 DB 操作表发现不了
*/

// Redis缓存对账任务（每日凌晨执行）
type CacheRepairJob struct {
	db          *gorm.DB
	cache       cache.InventoryCache
	redisClient redis.Cmdable // 需要原生client执行SCAN命令
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

func (j *CacheRepairJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// 分布式锁
	lockKey := "lock:cache_repair:daily"
	locked, err := j.redisClient.SetNX(ctx, lockKey, "1", 60*time.Minute).Result()
	if err != nil || !locked {
		j.logger.Info("对账任务已在其他实例执行，跳过")
		return nil
	}
	defer j.redisClient.Del(ctx, lockKey)

	j.logger.Info("开始执行Redis对账")

	// 1. 一次性扫描所有 reserve:* keys，构建预扣量 map
	// 避免每个商品都全量扫描，O(N) → O(1) 查询
	reservedMap, err := j.buildReservedMap(ctx)
	if err != nil {
		j.logger.Error("构建预扣量map失败", logger.Error(err))
		return err
	}
	j.logger.Info("预扣记录扫描完成", logger.Int("reserveKeyCount", len(reservedMap)))

	// 2. 扫描 Redis 所有 stock:* keys，收集有缓存的商品ID
	productIDs, err := j.scanStockKeys(ctx)
	if err != nil {
		j.logger.Error("扫描Redis stock keys失败", logger.Error(err))
		return err
	}

	if len(productIDs) == 0 {
		j.logger.Info("无需对账：Redis中无库存缓存")
		return nil
	}

	j.logger.Info("开始对账", logger.Int("productCount", len(productIDs)))

	// 3. 逐个检查修复（预扣量直接从 map 查，O(1)）
	repairCount := 0
	var repairErrors []string

	for _, productID := range productIDs {
		if err := j.repairProduct(ctx, productID, reservedMap); err != nil {
			repairErrors = append(repairErrors, fmt.Sprintf("productID:%d error:%v", productID, err))
		} else {
			repairCount++
		}
	}

	// 3. 结果统计
	if len(productIDs) > 0 {
		repairRate := float64(repairCount) / float64(len(productIDs)) * 100
		if repairRate > 5 { // 修复率>5%，可能系统性问题，触发告警
			j.logger.Error("对账发现大量差异，请人工介入",
				logger.Int("repairCount", repairCount),
				logger.Int("totalProducts", len(productIDs)),
				logger.String("repairRate", fmt.Sprintf("%.2f%%", repairRate)))
		} else if repairCount > 0 {
			j.logger.Warn("对账完成", logger.Int("repairCount", repairCount))
		} else {
			j.logger.Info("对账通过，Redis与DB一致")
		}
	}

	if len(repairErrors) > 0 {
		j.logger.Warn("对账过程有失败", logger.Int("errorCount", len(repairErrors)))
	}

	return nil
}

// 扫描 Redis 所有 stock:* keys，返回商品ID列表
func (j *CacheRepairJob) scanStockKeys(ctx context.Context) ([]int64, error) {
	var productIDs []int64
	var cursor uint64

	for {
		// SCAN 每次100个key，避免阻塞 Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "stock:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN失败: %w", err)
		}

		// 从 key 中提取商品ID（格式：stock:{productID}）
		for _, key := range keys {
			// key = "stock:123" → productID = 123
			if len(key) > 6 { // len("stock:") = 6
				if id, err := strconv.ParseInt(key[6:], 10, 64); err == nil {
					productIDs = append(productIDs, id)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break // 扫描完成
		}
	}

	return productIDs, nil
}

// 修复单个商品的Redis预库存
// reservedMap: 预先构建的 productID → 预扣总量 映射
func (j *CacheRepairJob) repairProduct(ctx context.Context, productID int64, reservedMap map[int64]int64) error {
	// 1. 查询Redis当前预库存
	// SCAN 扫到 key 后，Get 仍可能失败（竞态条件：key 被删除/过期）
	redisStockStr, err := j.cache.Get(ctx, repository.StockKey(productID))
	if err != nil {
		// key 可能在扫描后被删除/过期，属于正常情况，跳过
		j.logger.Debug("Redis key已不存在，跳过",
			logger.Int64("productID", productID),
			logger.Error(err))
		return nil
	}
	redisStock, _ := strconv.ParseInt(redisStockStr, 10, 64)

	// 2. 查询DB库存（真实库存）
	var dbStock int64
	err = j.db.WithContext(ctx).
		Table("inventories").
		Where("product_id = ?", productID).
		Pluck("stock", &dbStock).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 商品已删除，但Redis还有数据 → 删除Redis
			j.redisClient.Del(ctx, repository.StockKey(productID))
			return nil
		}
		return fmt.Errorf("查询DB失败: %w", err)
	}

	// 3. 从 map 获取预扣量（O(1)）
	reservedQty := reservedMap[productID]

	// 4. 计算期望的Redis预库存 = DB库存 - 预扣中的量
	expectedRedisStock := dbStock - reservedQty

	// 5. 检查是否一致
	if redisStock == expectedRedisStock {
		return nil // 一致，无需修复
	}

	// 6. 修复：写入正确的值
	j.logger.Warn("Redis预库存不一致，正在修复",
		logger.Int64("productID", productID),
		logger.Int64("dbStock", dbStock),
		logger.Int64("reservedQty", reservedQty),
		logger.Int64("expectedRedis", expectedRedisStock),
		logger.Int64("actualRedis", redisStock))
	_, err = j.cache.Set(ctx, repository.StockKey(productID), strconv.FormatInt(expectedRedisStock, 10), 0)
	if err != nil {
		return fmt.Errorf("修复Redis失败: %w", err)
	}

	return nil
}

// 一次性扫描所有 reserve:* keys，构建 productID → 预扣总量 映射
// 注意：已释放的预扣记录要排除（检查对应的 release key 是否存在）
func (j *CacheRepairJob) buildReservedMap(ctx context.Context) (map[int64]int64, error) {
	reservedMap := make(map[int64]int64)

	var cursor uint64
	for {
		// SCAN 每次100个 key，避免阻塞 Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "reserve:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN失败: %w", err)
		}

		// 对每个 reserve key，获取其中所有商品的预扣量
		for _, key := range keys {
			// 检查是否已释放：reserve:xxx → 检查 release:xxx 是否存在
			// key 格式: "reserve:{operationID}"
			operationID := key[8:] // len("reserve:") = 8
			releaseKey := "release:" + operationID

			exists, err := j.redisClient.Exists(ctx, releaseKey).Result()
			if err != nil {
				j.logger.Warn("检查release key失败", logger.String("releaseKey", releaseKey), logger.Error(err))
				continue
			}
			if exists > 0 {
				// 已释放，跳过这个预扣记录
				continue
			}

			// HGETALL 获取该预扣记录的所有 productID → quantity
			result, err := j.redisClient.HGetAll(ctx, key).Result()
			if err != nil {
				j.logger.Warn("读取预扣记录失败", logger.String("key", key), logger.Error(err))
				continue
			}

			// 累加到 map
			for productIDStr, qtyStr := range result {
				productID, err := strconv.ParseInt(productIDStr, 10, 64)
				if err != nil {
					continue
				}
				qty, _ := strconv.ParseInt(qtyStr, 10, 64)
				reservedMap[productID] += abs64(qty) // 预扣量存的是负数，取绝对值
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return reservedMap, nil
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
