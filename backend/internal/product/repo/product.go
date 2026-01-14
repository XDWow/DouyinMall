package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// =============================================================================
// Canal + MQ 模式实现（V2 版本）
// =============================================================================
//
// 与 CacheAside 版本的唯一区别：
//   - 写操作（Create/Update/Delete）不管缓存，由 Canal 消费者异步处理
//   - 其他功能完全一致
//
// 缓存策略亮点（继承自 CacheAside）：
//   - 分离缓存：基本信息 30min，价格/库存 1min
//   - 精准查询：价格/库存 miss 时只查 SELECT price, stock
//   - singleflight：热点数据防击穿
//   - Pipeline 预热：批量操作减少 RTT
//   - 穿透防护：空数组也缓存
//   - 降级策略：ctx.Value("downgrade") 控制
//
// Canal 架构亮点：
//   - 代码零侵入：业务代码不关心缓存失效，职责单一
//   - 扩展性强：新增下游（ES、推荐）只需加消费者
//   - 可靠性高：基于 binlog 保证消息不丢失
//   - 可观测强：Canal 提供同步进度监控
//   - 解耦彻底：缓存、ES、推荐等各自独立消费
// =============================================================================

type CachedProductRepo struct {
	dao    dao.ProductDao
	cache  cache.ProductCache
	logger logger.LoggerV1

	listCacheTTL        time.Duration
	detailBasicTTL      time.Duration
	detailPriceStockTTL time.Duration
	preloadTTL          time.Duration

	sf singleflight.Group
}

func NewCachedProductRepo(dao dao.ProductDao, cache cache.ProductCache, l logger.LoggerV1) ProductRepo {
	return &CachedProductRepo{
		dao:    dao,
		cache:  cache,
		logger: l,

		listCacheTTL:        3 * time.Minute,
		detailBasicTTL:      30 * time.Minute,
		detailPriceStockTTL: 1 * time.Minute,
		preloadTTL:          2 * time.Minute,
	}
}

func (r *CachedProductRepo) ListProducts(ctx context.Context, page, pageSize int64, category string) (products []domain.Product, err error) {
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

func (r *CachedProductRepo) preloadProductDetails(ctx context.Context, products []domain.Product) {
	preloadCount := 3
	if len(products) < preloadCount {
		preloadCount = len(products)
	}

	var items []cache.CacheItem

	for i := 0; i < preloadCount; i++ {
		product := products[i]
		basicKey := DetailBasicKey(product.ID)
		priceStockKey := PriceStockKey(product.ID)

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

		needPriceStock := false
		_, err = r.cache.Get(ctx, priceStockKey)
		if err != nil {
			needPriceStock = true
		} else {
			ttl, err := r.cache.GetTTL(ctx, priceStockKey)
			if err != nil || ttl <= 30*time.Second {
				needPriceStock = true
			}
		}
		if needPriceStock {
			items = append(items, cache.CacheItem{
				Key:   priceStockKey,
				Value: PriceStock{Price: product.Price, Stock: product.Stock},
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

func (r *CachedProductRepo) GetProduct(ctx context.Context, id int64) (product domain.Product, err error) {
	basicKey := DetailBasicKey(id)
	priceStockKey := PriceStockKey(id)

	basicData, basicErr := r.cache.Get(ctx, basicKey)
	priceStockData, priceStockErr := r.cache.Get(ctx, priceStockKey)

	if basicErr == nil && priceStockErr == nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			var ps PriceStock
			if err := json.Unmarshal(priceStockData, &ps); err == nil {
				product.Price = ps.Price
				product.Stock = ps.Stock
				return product, nil
			}
		}
	}

	if basicErr == nil && priceStockErr != nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			price, stock, err := r.dao.FindPriceStock(ctx, id)
			if err != nil {
				r.logger.Error("查询价格/库存失败，使用降级数据",
					logger.Int64("product_id", id),
					logger.Error(err))
				product.Price = 0
				product.Stock = 0
				return product, nil
			}
			product.Price = price
			product.Stock = stock

			r.cachePriceStock(ctx, id, price, stock, r.detailPriceStockTTL)
			return product, nil
		}
	}

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
	r.cachePriceStock(ctx, id, product.Price, product.Stock, r.detailPriceStockTTL)

	return product, nil
}

func (r *CachedProductRepo) cacheProductBasicDetail(ctx context.Context, product domain.Product, ttl time.Duration) {
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

func (r *CachedProductRepo) cachePriceStock(ctx context.Context, id int64, price int64, stock int64, ttl time.Duration) {
	key := PriceStockKey(id)
	ps := PriceStock{Price: price, Stock: stock}
	if err := r.cache.SetWithTTL(ctx, key, ps, ttl); err != nil {
		r.logger.Error("缓存价格/库存失败", logger.Int64("product_id", id), logger.Error(err))
	}
}

func (r *CachedProductRepo) CreateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}
	id, err := r.dao.Insert(ctx, entity)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// 与 CacheAside 的唯一区别：不删缓存，由 Canal 消费者处理，这里逻辑非常简单
func (r *CachedProductRepo) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	err = r.dao.Update(ctx, entity)
	if err != nil {
		return 0, err
	}
	return product.ID, nil
}

func (r *CachedProductRepo) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	err = r.dao.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	return nil
}

func (r *CachedProductRepo) domainToEntity(d domain.Product) (dao.Product, error) {
	entity := dao.Product{
		ID:           d.ID,
		Name:         d.Name,
		Price:        d.Price,
		Stock:        d.Stock,
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

func (r *CachedProductRepo) entityToDomain(e dao.Product) (domain.Product, error) {
	domainProduct := domain.Product{
		ID:         e.ID,
		Name:       e.Name,
		Price:      e.Price,
		Stock:      e.Stock,
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
