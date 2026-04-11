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
	缓存问题往往不是「在线修 Redis」能解决的，而是业务流程与最终一致性：
	以数据库库存为准，在 in-flight 预扣结束后，从 DB 重建 Redis 中的库存视图。
*/

// 以下为历史设计备忘（定时对账思路）
/*
定时任务设计要点：

核心原则：
1. Redis 展示库存 = DB 库存 - 当前仍在预扣中的量
2. 扫描 Redis 中已有库存缓存的商品，检查是否与上述一致

对账策略：
- 频率：每日凌晨
- 范围：扫描 Redis 所有 stock:* keys（有缓存的商品）
- 分布式锁：防止多实例并发执行
- 分批：SCAN 每次 100 个 key，避免长时间阻塞 Redis
- 监控告警：修复商品数 > 总商品数 5% 时触发告警

为何扫 Redis 而不是查 DB 操作表：
- DB 操作表主要记录 CommitStock / RefundStock
- ReserveStock / ReleaseStock 不写 DB，只动 Redis
- 因此预扣/释放失败导致的不一致，查 DB 操作表发现不了
*/

// CacheRepairJob Redis 缓存对账任务（例如每日凌晨执行）
type CacheRepairJob struct {
	db          *gorm.DB
	cache       cache.InventoryCache
	redisClient redis.Cmdable // 需要原生 Client 以执行 SCAN
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

	j.logger.Info("开始执行 Redis 对账")

	// 1. 一次性扫描所有 reserve:* keys，构建预扣量 map（避免每个商品全量扫描，O(N) 查询 -> O(1) 查 map）
	reservedMap, err := j.buildReservedMap(ctx)
	if err != nil {
		j.logger.Error("构建预扣量 map 失败", logger.Error(err))
		return err
	}
	j.logger.Info("预扣记录扫描完成", logger.Int("reserveKeyCount", len(reservedMap)))

	// 2. 扫描 Redis 所有 stock:* keys，收集有缓存的商品 ID
	productIDs, err := j.scanStockKeys(ctx)
	if err != nil {
		j.logger.Error("扫描 Redis stock keys 失败", logger.Error(err))
		return err
	}

	if len(productIDs) == 0 {
		j.logger.Info("无需对账：Redis 中无库存缓存")
		return nil
	}

	j.logger.Info("开始对账", logger.Int("productCount", len(productIDs)))

	// 3. 逐个检查修复（预扣量直接从 map 取，O(1)）
	repairCount := 0
	var repairErrors []string

	for _, productID := range productIDs {
		if err := j.repairProduct(ctx, productID, reservedMap); err != nil {
			repairErrors = append(repairErrors, fmt.Sprintf("productID:%d error:%v", productID, err))
		} else {
			repairCount++
		}
	}

	// 4. 结果统计
	if len(productIDs) > 0 {
		repairRate := float64(repairCount) / float64(len(productIDs)) * 100
		if repairRate > 5 { // 修复率 >5%，可能系统性问题，触发告警
			j.logger.Error("对账发现大量差异，请人工介入",
				logger.Int("repairCount", repairCount),
				logger.Int("totalProducts", len(productIDs)),
				logger.String("repairRate", fmt.Sprintf("%.2f%%", repairRate)))
		} else if repairCount > 0 {
			j.logger.Warn("对账完成", logger.Int("repairCount", repairCount))
		} else {
			j.logger.Info("对账通过：Redis 与 DB 一致")
		}
	}

	if len(repairErrors) > 0 {
		j.logger.Warn("对账过程有失败", logger.Int("errorCount", len(repairErrors)))
	}

	return nil
}

// scanStockKeys 扫描 Redis 所有 stock:* keys，返回商品 ID 列表
func (j *CacheRepairJob) scanStockKeys(ctx context.Context) ([]int64, error) {
	var productIDs []int64
	var cursor uint64

	for {
		// SCAN 每次 100 个 key，避免阻塞 Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "stock:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN 失败: %w", err)
		}

		// 从 key 解析商品 ID（格式：stock:{productID}）
		for _, key := range keys {
			// key = "stock:123" -> productID = 123
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

// repairProduct 修复单个商品的 Redis 预展示库存
// reservedMap: 预先构建的 productID -> 总预扣量
func (j *CacheRepairJob) repairProduct(ctx context.Context, productID int64, reservedMap map[int64]int64) error {
	// 1. 读 Redis 当前展示库存
	// SCAN 扫到 key 后 Get 仍可能失败（竞态：key 被删或过期）
	redisStockStr, err := j.cache.Get(ctx, repository.StockKey(productID))
	if err != nil {
		// key 可能在扫描后被删/过期，属正常情况，跳过
		j.logger.Debug("Redis key 已不存在，跳过",
			logger.Int64("productID", productID),
			logger.Error(err))
		return nil
	}
	redisStock, _ := strconv.ParseInt(redisStockStr, 10, 64)

	// 2. 查 DB 真实库存
	var dbStock int64
	err = j.db.WithContext(ctx).
		Table("inventories").
		Where("product_id = ?", productID).
		Pluck("stock", &dbStock).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 商品已删但 Redis 仍有数据 -> 删 Redis
			j.redisClient.Del(ctx, repository.StockKey(productID))
			return nil
		}
		return fmt.Errorf("查询 DB 失败: %w", err)
	}

	// 3. 从 map 取预扣量（O(1)）
	reservedQty := reservedMap[productID]

	// 4. 期望的 Redis 展示库存 = DB 库存 - 预扣中的量
	expectedRedisStock := dbStock - reservedQty

	// 5. 是否一致
	if redisStock == expectedRedisStock {
		return nil // 一致，无需修复
	}

	// 6. 修复：写入正确值
	j.logger.Warn("Redis 展示库存与期望值不一致，正在修复",
		logger.Int64("productID", productID),
		logger.Int64("dbStock", dbStock),
		logger.Int64("reservedQty", reservedQty),
		logger.Int64("expectedRedis", expectedRedisStock),
		logger.Int64("actualRedis", redisStock))
	_, err = j.cache.Set(ctx, repository.StockKey(productID), strconv.FormatInt(expectedRedisStock, 10), 0)
	if err != nil {
		return fmt.Errorf("修复 Redis 失败: %w", err)
	}

	return nil
}

// buildReservedMap 一次性扫描所有 reserve:* keys，构建 productID -> 总预扣量
// 注意：已释放的预扣要排除（检查对应 release key 是否存在）
func (j *CacheRepairJob) buildReservedMap(ctx context.Context) (map[int64]int64, error) {
	reservedMap := make(map[int64]int64)

	var cursor uint64
	for {
		// SCAN 每次 100 个 key，避免阻塞 Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "reserve:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN 失败: %w", err)
		}

		for _, key := range keys {
			// 是否已释放：reserve:xxx -> 检查 release:xxx 是否存在
			// key 格式: "reserve:{operationID}"
			operationID := key[8:] // len("reserve:") = 8
			releaseKey := "release:" + operationID

			exists, err := j.redisClient.Exists(ctx, releaseKey).Result()
			if err != nil {
				j.logger.Warn("检查 release key 失败", logger.String("releaseKey", releaseKey), logger.Error(err))
				continue
			}
			if exists > 0 {
				// 已释放，跳过该预扣记录
				continue
			}

			result, err := j.redisClient.HGetAll(ctx, key).Result()
			if err != nil {
				j.logger.Warn("读取预扣记录失败", logger.String("key", key), logger.Error(err))
				continue
			}

			for productIDStr, qtyStr := range result {
				productID, err := strconv.ParseInt(productIDStr, 10, 64)
				if err != nil {
					continue
				}
				qty, _ := strconv.ParseInt(qtyStr, 10, 64)
				reservedMap[productID] += abs64(qty) // 预扣在 Hash 里存负数，取绝对值累加
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
