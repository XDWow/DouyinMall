package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// =============================================================================
// Cache-Aside 模式实现（V1 版本）
// =============================================================================
//
// 架构特点：
//   - 业务代码直接管理缓存（读写删）
//   - 更新/删除时主动失效缓存（延迟双删）
//   - 适用于单 Redis、无 MQ 的简单场景
//
// 缓存策略：
//   - 列表缓存：短 TTL（3分钟），热点数据 singleflight 防击穿
//   - 详情缓存：基本信息 30 分钟，价格/库存状态 1 分钟（分离存储）
//   - 预热缓存：2 分钟（预测性质）
//
// 一致性保障：
//   - 延迟双删：解决并发读写导致的脏缓存问题（主从延迟会加剧）
//   - 列表缓存不主动删除，依赖短 TTL 自然过期（变化频繁，删除成本高）
//   - 详情缓存主动删除 + TTL 兜底（删除失败时 TTL 保证最终一致）
//
// 缺点：
//   - 代码侵入：每处写操作都要管理缓存
//   - 扩展性差：新增缓存（如 ES、多 Redis）需要修改代码
//   - 可靠性弱：删缓存失败会导致不一致
//
// 亮点：
//	分离缓存	基本信息 30min，价格/库存状态 1min，独立 TTL
//	精准查询	价格/库存状态 miss 时只查 SELECT price, in_stock，减少网络传输
//	singleflight	热点数据（前3页）防缓存击穿
//	缓存预热	列表查询后预热前3个商品详情
//	延迟双删	更新后立即删 + 延迟1秒再删，解决并发脏缓存
//	穿透防护	空数组也缓存
//	降级策略	ctx.Value("downgrade") 控制

// =============================================================================

type CacheAsideProductRepo struct {
	dao    dao.ProductDao
	cache  cache.ProductCache
	logger logger.LoggerV1

	// TTL 配置
	listCacheTTL        time.Duration
	detailBasicTTL      time.Duration
	detailPriceStockTTL time.Duration
	preloadTTL          time.Duration

	sf singleflight.Group
}

func NewCacheAsideProductRepo(dao dao.ProductDao, cache cache.ProductCache, l logger.LoggerV1) ProductRepo {
	return &CacheAsideProductRepo{
		dao:    dao,
		cache:  cache,
		logger: l,

		listCacheTTL:        3 * time.Minute,  // 列表：3分钟（不主动删除，依赖自然过期）
		detailBasicTTL:      30 * time.Minute, // 基本信息：30分钟（主动删除 + TTL 兜底）
		detailPriceStockTTL: 1 * time.Minute,  // 价格/库存状态：1分钟（主动删除 + TTL 兜底，最多 1 分钟恢复一致）
		preloadTTL:          2 * time.Minute,  // 预热：2分钟（预测失败快速过期）
	}
}

func ListKey(category string, page int64) string {
	return fmt.Sprintf("product:list:%s:%d", category, page)
}

func DetailBasicKey(id int64) string {
	return fmt.Sprintf("product:detail:basic:%d", id)
}

func PriceInStockKey(id int64) string {
	return fmt.Sprintf("product:detail:price_in_stock:%d", id)
}

func (r *CacheAsideProductRepo) ListProducts(ctx context.Context, page, pageSize int64, category string) (products []domain.Product, err error) {
	// 热点数据（前3页）使用 singleflight 防止缓存击穿
	if page <= 3 {
		key := ListKey(category, page)
		data, err := r.cache.Get(ctx, key)
		if err == nil {
			var cached []domain.Product
			if err := json.Unmarshal(data, &cached); err == nil {
				if len(cached) > 0 {
					go r.preloadProductDetails(context.Background(), cached)
				}
				if int64(len(cached)) > pageSize {
					return cached[:pageSize], nil
				}
				return cached, nil
			}
		}

		// singleflight：合并相同 key 的并发请求，只有一个请求查数据库
		val, err, _ := r.sf.Do(key, func() (interface{}, error) {
			ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
			if err != nil {
				return nil, err
			}

			res := make([]domain.Product, 0, len(ps))
			for _, p := range ps {
				domainProduct, err := r.entityToDomain(p)
				if err != nil {
					r.logger.Error("转换商品数据失败", logger.Error(err))
					continue
				}
				res = append(res, domainProduct)
			}

			// 缓存结果（空数组也缓存，防止穿透）
			_ = r.cache.SetWithTTL(ctx, key, res, r.listCacheTTL)
			return res, nil
		})

		if err != nil {
			return nil, err
		}

		res := val.([]domain.Product)
		if len(res) > 0 {
			go r.preloadProductDetails(context.Background(), res)
		}
		if int64(len(res)) > pageSize {
			return res[:pageSize], nil
		}
		return res, nil
	}

	// 非热点数据，直接查数据库
	ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		domainProduct, err := r.entityToDomain(p)
		if err != nil {
			r.logger.Error("转换商品数据失败", logger.Error(err))
			continue
		}
		products = append(products, domainProduct)
	}
	return products, nil
}

// 用户浏览列表后大概率点击前几个商品，提前缓存减少延迟
func (r *CacheAsideProductRepo) preloadProductDetails(ctx context.Context, products []domain.Product) {
	preloadCount := 3
	if len(products) < preloadCount {
		preloadCount = len(products)
	}

	var items []cache.CacheItem

	for i := 0; i < preloadCount; i++ {
		product := products[i]
		basicKey := DetailBasicKey(product.ID)
		priceInStockKey := PriceInStockKey(product.ID)

		// 检查基本信息缓存，不存在或快过期则预热
		needBasic := false
		_, err := r.cache.Get(ctx, basicKey)
		if err != nil {
			needBasic = true
		} else {
			ttl, err := r.cache.GetTTL(ctx, basicKey)
			if err != nil || ttl <= time.Minute {
				needBasic = true
			}
		}
		if needBasic {
			items = append(items, cache.CacheItem{
				Key: basicKey,
				Value: domain.Product{
					ID:           product.ID,
					Name:         product.Name,
					Description:  product.Description,
					Picture:      product.Picture,
					SlideImgs:    product.SlideImgs,
					Categories:   product.Categories,
					MerchantID:   product.MerchantID,
					MerchantName: product.MerchantName,
				},
				TTL: r.preloadTTL,
			})
		}

		// 检查价格/库存状态缓存，不存在或快过期则预热
		needPriceInStock := false
		_, err = r.cache.Get(ctx, priceInStockKey)
		if err != nil {
			needPriceInStock = true
		} else {
			ttl, err := r.cache.GetTTL(ctx, priceInStockKey)
			if err != nil || ttl <= 30*time.Second {
				needPriceInStock = true
			}
		}
		if needPriceInStock {
			items = append(items, cache.CacheItem{
				Key:   priceInStockKey,
				Value: PriceInStock{Price: product.Price, InStock: product.InStock},
				TTL:   r.preloadTTL,
			})
		}
	}

	if len(items) > 0 {
		if err := r.cache.BatchSetWithTTL(ctx, items); err != nil {
			r.logger.Error("批量预缓存商品失败", logger.Error(err))
		}
	}
}

func (r *CacheAsideProductRepo) GetProduct(ctx context.Context, id int64) (product domain.Product, err error) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	// 1. 尝试从缓存获取
	basicData, basicErr := r.cache.Get(ctx, basicKey)
	priceInStockData, priceInStockErr := r.cache.Get(ctx, priceInStockKey)

	// 情况1：都命中，直接返回
	if basicErr == nil && priceInStockErr == nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			var ps PriceInStock
			if err := json.Unmarshal(priceInStockData, &ps); err == nil {
				product.Price = ps.Price
				product.InStock = ps.InStock
				return product, nil
			}
		}
	}

	// 情况2：基本信息命中，价格/库存状态未命中（分离存储的优势）
	// 优化点：只查 price, in_stock 两个字段，不查完整商品
	if basicErr == nil && priceInStockErr != nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			price, inStock, err := r.dao.FindPriceInStock(ctx, id)
			if err != nil {
				r.logger.Error("查询价格/库存状态失败，使用降级数据",
					logger.Int64("product_id", id),
					logger.Error(err))
				product.Price = 0
				product.InStock = false
				return product, nil
			}
			product.Price = price
			product.InStock = inStock

			r.cachePriceInStock(ctx, id, price, inStock, r.detailPriceStockTTL)
			return product, nil
		}
	}

	// 情况3：都未命中，或基本信息没中，只中了价格，直接查库
	// 降级
	if ctx.Value("downgrade") == "true" {
		return domain.Product{}, errors.New("降级策略：缓存未命中，跳过数据库查询")
	}
	pe, err := r.dao.FindByID(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	product, err = r.entityToDomain(pe)
	if err != nil {
		return domain.Product{}, err
	}

	r.cacheProductBasicDetail(ctx, product, r.detailBasicTTL)
	r.cachePriceInStock(ctx, id, product.Price, product.InStock, r.detailPriceStockTTL)

	return product, nil
}

type PriceInStock struct {
	Price   int64 `json:"price"`    // 单位：分
	InStock bool  `json:"in_stock"` // 是否有货
}

func (r *CacheAsideProductRepo) cacheProductBasicDetail(ctx context.Context, product domain.Product, ttl time.Duration) {
	key := DetailBasicKey(product.ID)
	basicProduct := domain.Product{
		ID:           product.ID,
		Name:         product.Name,
		Description:  product.Description,
		Picture:      product.Picture,
		SlideImgs:    product.SlideImgs,
		Categories:   product.Categories,
		MerchantID:   product.MerchantID,
		MerchantName: product.MerchantName,
	}
	if err := r.cache.SetWithTTL(ctx, key, basicProduct, ttl); err != nil {
		r.logger.Error("缓存商品基本信息失败", logger.Int64("product_id", product.ID), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) cachePriceInStock(ctx context.Context, id int64, price int64, inStock bool, ttl time.Duration) {
	key := PriceInStockKey(id)
	ps := PriceInStock{Price: price, InStock: inStock}
	if err := r.cache.SetWithTTL(ctx, key, ps, ttl); err != nil {
		r.logger.Error("缓存价格/库存状态失败", logger.Int64("product_id", id), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) CreateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}
	id, err := r.dao.Insert(ctx, entity)
	if err != nil {
		return 0, err
	}

	product.ID = id
	r.cacheProductBasicDetail(ctx, product, r.detailBasicTTL)
	r.cachePriceInStock(ctx, id, product.Price, product.InStock, r.detailPriceStockTTL)
	return id, nil
}

// 延迟双删，解决并发读写导致的脏缓存问题
func (r *CacheAsideProductRepo) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	// 1. 更新数据库
	err = r.dao.Update(ctx, entity)
	if err != nil {
		return 0, err
	}

	// 2. 立即删除缓存（第一次）：让后续请求查库获取新数据
	r.deleteProductCache(ctx, product.ID)

	// 3. 延迟后再删除（第二次）：清除并发读写产生的脏缓存
	// 场景：请求1查缓存Miss,查数据库（旧）,请求2更新数据库（新），请求2删缓存，请求1写旧数据到缓存
	// 第二次延迟删除可以清空这个旧数据
	go r.delayedDelete(product.ID)

	return product.ID, nil
}

// 这里用的是软删除：实际也是更新字段，一样要延迟双删
func (r *CacheAsideProductRepo) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	err = r.dao.Delete(ctx, id, userID)
	if err != nil {
		return err
	}

	r.deleteProductCache(ctx, id)

	go r.delayedDelete(id)

	return nil
}

// deleteProductCache 删除商品详情缓存，不删除列表缓存
// 原因：
//  1. 列表缓存变化频繁（新建/更新/删除商品都会影响多个列表页），开销大
//  2. 列表相对商品详情实时性要求较低，通过短 TTL（3分钟）自然过期即可，主动删收益小
func (r *CacheAsideProductRepo) deleteProductCache(ctx context.Context, id int64) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	if err := r.cache.Delete(ctx, basicKey); err != nil {
		r.logger.Error("删除基本信息缓存失败", logger.Int64("product_id", id), logger.Error(err))
	}
	if err := r.cache.Delete(ctx, priceInStockKey); err != nil {
		r.logger.Error("删除价格/库存状态缓存失败", logger.Int64("product_id", id), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) delayedDelete(id int64) {
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r.deleteProductCache(ctx, id)
	r.logger.Info("延迟双删完成", logger.Int64("product_id", id))
}

func (r *CacheAsideProductRepo) domainToEntity(d domain.Product) (dao.Product, error) {
	entity := dao.Product{
		ID:           d.ID,
		Name:         d.Name,
		Price:        d.Price,
		InStock:      d.InStock,
		MerchantID:   d.MerchantID,
		MerchantName: sql.NullString{String: d.MerchantName, Valid: d.MerchantName != ""},
		Description:  sql.NullString{String: d.Description, Valid: d.Description != ""},
		Picture:      sql.NullString{String: d.Picture, Valid: d.Picture != ""},
	}

	if len(d.SlideImgs) > 0 {
		data, err := json.Marshal(d.SlideImgs)
		if err != nil {
			return dao.Product{}, err
		}
		entity.SlideImgs = string(data)
	} else {
		entity.SlideImgs = "[]"
	}

	if len(d.Categories) > 0 {
		data, err := json.Marshal(d.Categories)
		if err != nil {
			return dao.Product{}, err
		}
		entity.Categories = string(data)
	} else {
		entity.Categories = "[]"
	}

	return entity, nil
}

func (r *CacheAsideProductRepo) entityToDomain(e dao.Product) (domain.Product, error) {
	domainProduct := domain.Product{
		ID:         e.ID,
		Name:       e.Name,
		Price:      e.Price,
		InStock:    e.InStock,
		MerchantID: e.MerchantID,
	}

	if e.Description.Valid {
		domainProduct.Description = e.Description.String
	}
	if e.Picture.Valid {
		domainProduct.Picture = e.Picture.String
	}
	if e.MerchantName.Valid {
		domainProduct.MerchantName = e.MerchantName.String
	}

	if e.SlideImgs != "" && e.SlideImgs != "[]" {
		var imgs []string
		if err := json.Unmarshal([]byte(e.SlideImgs), &imgs); err != nil {
			return domain.Product{}, err
		}
		domainProduct.SlideImgs = imgs
	} else {
		domainProduct.SlideImgs = []string{}
	}

	if e.Categories != "" && e.Categories != "[]" {
		var categories []string
		if err := json.Unmarshal([]byte(e.Categories), &categories); err != nil {
			return domain.Product{}, err
		}
		domainProduct.Categories = categories
	} else {
		domainProduct.Categories = []string{}
	}

	return domainProduct, nil
}
