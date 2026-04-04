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
	鍏滃簳鎼炰笉浜嗭紝搴撳瓨鍏滃簳涓嶆槸鍦ㄧ嚎淇闂锛岃€屾槸浜嬫晠澶勭悊娴佺▼
	閫氳繃鍐荤粨搴撳瓨鍐欏叆锛岀瓑寰?in-flight reservation 缁撴潫鍚庯紝浠?DB 涓轰簨瀹炴簮閲嶅缓 Redis 鏁版嵁
*/

// 涓嬮潰鏄箣鍓嶉敊璇殑鎯虫硶
/*
瀹氭椂浠诲姟璁捐锛?

鏍稿績鍘熷垯锛?
1. Redis棰勫簱瀛?= DB搴撳瓨 - 姝ｅ湪棰勬墸涓殑閲?
2. 鎵弿 Redis 涓湁搴撳瓨缂撳瓨鐨勫晢鍝侊紝妫€鏌ユ槸鍚︽纭?

瀵硅处绛栫暐锛?
- 棰戠巼锛氭瘡澶╁噷鏅?鐐?
- 鑼冨洿锛氭壂鎻?Redis 鎵€鏈?stock:* keys锛堟湁缂撳瓨鐨勫晢鍝侊級
- 鍒嗗竷寮忛攣锛氶槻姝㈠瀹炰緥骞跺彂鎵ц
- 鍒嗘壒澶勭悊锛歋CAN 姣忔100涓猭ey锛岄伩鍏嶉樆濉?Redis
- 鐩戞帶鍛婅锛氫慨澶嶅晢鍝佹暟 > 5% 瑙﹀彂鍛婅

涓轰粈涔堟壂鎻?Redis 鑰屼笉鏄煡 DB 鎿嶄綔琛細
- DB 鎿嶄綔琛ㄥ彧璁板綍 CommitStock/RefundStock
- ReserveStock/ReleaseStock 涓嶅啓 DB锛屽彧鎿嶄綔 Redis
- 鎵€浠ラ鎵?閲婃斁澶辫触瀵艰嚧鐨勪笉涓€鑷达紝鏌?DB 鎿嶄綔琛ㄥ彂鐜颁笉浜?
*/

// Redis缂撳瓨瀵硅处浠诲姟锛堟瘡鏃ュ噷鏅ㄦ墽琛岋級
type CacheRepairJob struct {
	db          *gorm.DB
	cache       cache.InventoryCache
	redisClient redis.Cmdable // 闇€瑕佸師鐢焎lient鎵цSCAN鍛戒护
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

	// 鍒嗗竷寮忛攣
	lockKey := "lock:cache_repair:daily"
	locked, err := j.redisClient.SetNX(ctx, lockKey, "1", 60*time.Minute).Result()
	if err != nil || !locked {
		j.logger.Info("瀵硅处浠诲姟宸插湪鍏朵粬瀹炰緥鎵ц锛岃烦杩?)
		return nil
	}
	defer j.redisClient.Del(ctx, lockKey)

	j.logger.Info("寮€濮嬫墽琛孯edis瀵硅处")

	// 1. 涓€娆℃€ф壂鎻忔墍鏈?reserve:* keys锛屾瀯寤洪鎵ｉ噺 map
	// 閬垮厤姣忎釜鍟嗗搧閮藉叏閲忔壂鎻忥紝O(N) 鈫?O(1) 鏌ヨ
	reservedMap, err := j.buildReservedMap(ctx)
	if err != nil {
		j.logger.Error("鏋勫缓棰勬墸閲弇ap澶辫触", logger.Error(err))
		return err
	}
	j.logger.Info("棰勬墸璁板綍鎵弿瀹屾垚", logger.Int("reserveKeyCount", len(reservedMap)))

	// 2. 鎵弿 Redis 鎵€鏈?stock:* keys锛屾敹闆嗘湁缂撳瓨鐨勫晢鍝両D
	productIDs, err := j.scanStockKeys(ctx)
	if err != nil {
		j.logger.Error("鎵弿Redis stock keys澶辫触", logger.Error(err))
		return err
	}

	if len(productIDs) == 0 {
		j.logger.Info("鏃犻渶瀵硅处锛歊edis涓棤搴撳瓨缂撳瓨")
		return nil
	}

	j.logger.Info("寮€濮嬪璐?, logger.Int("productCount", len(productIDs)))

	// 3. 閫愪釜妫€鏌ヤ慨澶嶏紙棰勬墸閲忕洿鎺ヤ粠 map 鏌ワ紝O(1)锛?
	repairCount := 0
	var repairErrors []string

	for _, productID := range productIDs {
		if err := j.repairProduct(ctx, productID, reservedMap); err != nil {
			repairErrors = append(repairErrors, fmt.Sprintf("productID:%d error:%v", productID, err))
		} else {
			repairCount++
		}
	}

	// 3. 缁撴灉缁熻
	if len(productIDs) > 0 {
		repairRate := float64(repairCount) / float64(len(productIDs)) * 100
		if repairRate > 5 { // 淇鐜?5%锛屽彲鑳界郴缁熸€ч棶棰橈紝瑙﹀彂鍛婅
			j.logger.Error("瀵硅处鍙戠幇澶ч噺宸紓锛岃浜哄伐浠嬪叆",
				logger.Int("repairCount", repairCount),
				logger.Int("totalProducts", len(productIDs)),
				logger.String("repairRate", fmt.Sprintf("%.2f%%", repairRate)))
		} else if repairCount > 0 {
			j.logger.Warn("瀵硅处瀹屾垚", logger.Int("repairCount", repairCount))
		} else {
			j.logger.Info("瀵硅处閫氳繃锛孯edis涓嶥B涓€鑷?)
		}
	}

	if len(repairErrors) > 0 {
		j.logger.Warn("瀵硅处杩囩▼鏈夊け璐?, logger.Int("errorCount", len(repairErrors)))
	}

	return nil
}

// 鎵弿 Redis 鎵€鏈?stock:* keys锛岃繑鍥炲晢鍝両D鍒楄〃
func (j *CacheRepairJob) scanStockKeys(ctx context.Context) ([]int64, error) {
	var productIDs []int64
	var cursor uint64

	for {
		// SCAN 姣忔100涓猭ey锛岄伩鍏嶉樆濉?Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "stock:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN澶辫触: %w", err)
		}

		// 浠?key 涓彁鍙栧晢鍝両D锛堟牸寮忥細stock:{productID}锛?
		for _, key := range keys {
			// key = "stock:123" 鈫?productID = 123
			if len(key) > 6 { // len("stock:") = 6
				if id, err := strconv.ParseInt(key[6:], 10, 64); err == nil {
					productIDs = append(productIDs, id)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break // 鎵弿瀹屾垚
		}
	}

	return productIDs, nil
}

// 淇鍗曚釜鍟嗗搧鐨凴edis棰勫簱瀛?
// reservedMap: 棰勫厛鏋勫缓鐨?productID 鈫?棰勬墸鎬婚噺 鏄犲皠
func (j *CacheRepairJob) repairProduct(ctx context.Context, productID int64, reservedMap map[int64]int64) error {
	// 1. 鏌ヨRedis褰撳墠棰勫簱瀛?
	// SCAN 鎵埌 key 鍚庯紝Get 浠嶅彲鑳藉け璐ワ紙绔炴€佹潯浠讹細key 琚垹闄?杩囨湡锛?
	redisStockStr, err := j.cache.Get(ctx, repository.StockKey(productID))
	if err != nil {
		// key 鍙兘鍦ㄦ壂鎻忓悗琚垹闄?杩囨湡锛屽睘浜庢甯告儏鍐碉紝璺宠繃
		j.logger.Debug("Redis key宸蹭笉瀛樺湪锛岃烦杩?,
			logger.Int64("productID", productID),
			logger.Error(err))
		return nil
	}
	redisStock, _ := strconv.ParseInt(redisStockStr, 10, 64)

	// 2. 鏌ヨDB搴撳瓨锛堢湡瀹炲簱瀛橈級
	var dbStock int64
	err = j.db.WithContext(ctx).
		Table("inventories").
		Where("product_id = ?", productID).
		Pluck("stock", &dbStock).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 鍟嗗搧宸插垹闄わ紝浣哛edis杩樻湁鏁版嵁 鈫?鍒犻櫎Redis
			j.redisClient.Del(ctx, repository.StockKey(productID))
			return nil
		}
		return fmt.Errorf("鏌ヨDB澶辫触: %w", err)
	}

	// 3. 浠?map 鑾峰彇棰勬墸閲忥紙O(1)锛?
	reservedQty := reservedMap[productID]

	// 4. 璁＄畻鏈熸湜鐨凴edis棰勫簱瀛?= DB搴撳瓨 - 棰勬墸涓殑閲?
	expectedRedisStock := dbStock - reservedQty

	// 5. 妫€鏌ユ槸鍚︿竴鑷?
	if redisStock == expectedRedisStock {
		return nil // 涓€鑷达紝鏃犻渶淇
	}

	// 6. 淇锛氬啓鍏ユ纭殑鍊?
	j.logger.Warn("Redis棰勫簱瀛樹笉涓€鑷达紝姝ｅ湪淇",
		logger.Int64("productID", productID),
		logger.Int64("dbStock", dbStock),
		logger.Int64("reservedQty", reservedQty),
		logger.Int64("expectedRedis", expectedRedisStock),
		logger.Int64("actualRedis", redisStock))
	_, err = j.cache.Set(ctx, repository.StockKey(productID), strconv.FormatInt(expectedRedisStock, 10), 0)
	if err != nil {
		return fmt.Errorf("淇Redis澶辫触: %w", err)
	}

	return nil
}

// 涓€娆℃€ф壂鎻忔墍鏈?reserve:* keys锛屾瀯寤?productID 鈫?棰勬墸鎬婚噺 鏄犲皠
// 娉ㄦ剰锛氬凡閲婃斁鐨勯鎵ｈ褰曡鎺掗櫎锛堟鏌ュ搴旂殑 release key 鏄惁瀛樺湪锛?
func (j *CacheRepairJob) buildReservedMap(ctx context.Context) (map[int64]int64, error) {
	reservedMap := make(map[int64]int64)

	var cursor uint64
	for {
		// SCAN 姣忔100涓?key锛岄伩鍏嶉樆濉?Redis
		keys, nextCursor, err := j.redisClient.Scan(ctx, cursor, "reserve:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("SCAN澶辫触: %w", err)
		}

		// 瀵规瘡涓?reserve key锛岃幏鍙栧叾涓墍鏈夊晢鍝佺殑棰勬墸閲?
		for _, key := range keys {
			// 妫€鏌ユ槸鍚﹀凡閲婃斁锛歳eserve:xxx 鈫?妫€鏌?release:xxx 鏄惁瀛樺湪
			// key 鏍煎紡: "reserve:{operationID}"
			operationID := key[8:] // len("reserve:") = 8
			releaseKey := "release:" + operationID

			exists, err := j.redisClient.Exists(ctx, releaseKey).Result()
			if err != nil {
				j.logger.Warn("妫€鏌elease key澶辫触", logger.String("releaseKey", releaseKey), logger.Error(err))
				continue
			}
			if exists > 0 {
				// 宸查噴鏀撅紝璺宠繃杩欎釜棰勬墸璁板綍
				continue
			}

			// HGETALL 鑾峰彇璇ラ鎵ｈ褰曠殑鎵€鏈?productID 鈫?quantity
			result, err := j.redisClient.HGetAll(ctx, key).Result()
			if err != nil {
				j.logger.Warn("璇诲彇棰勬墸璁板綍澶辫触", logger.String("key", key), logger.Error(err))
				continue
			}

			// 绱姞鍒?map
			for productIDStr, qtyStr := range result {
				productID, err := strconv.ParseInt(productIDStr, 10, 64)
				if err != nil {
					continue
				}
				qty, _ := strconv.ParseInt(qtyStr, 10, 64)
				reservedMap[productID] += abs64(qty) // 棰勬墸閲忓瓨鐨勬槸璐熸暟锛屽彇缁濆鍊?
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


