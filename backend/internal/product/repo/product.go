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

type CacheAsideProductRepo struct {
	dao    dao.ProductDao
	cache  cache.ProductCache
	logger logger.LoggerV1

	listCacheTTL        time.Duration
	detailBasicTTL      time.Duration
	detailPriceStockTTL time.Duration
	preloadTTL          time.Duration

	sf singleflight.Group
}

type PriceInStock struct {
	SKUID    int64  `json:"sku_id"`
	Price    int64  `json:"price"`
	Currency string `json:"currency"`
	InStock  bool   `json:"in_stock"`
}

type cachedProductParts struct {
	basic      domain.Product
	basicFound bool

	priceStock PriceInStock
	priceFound bool
}

func NewCachedProductRepo(dao dao.ProductDao, cache cache.ProductCache, l logger.LoggerV1) ProductRepo {
	return &CacheAsideProductRepo{
		dao:                 dao,
		cache:               cache,
		logger:              l,
		listCacheTTL:        3 * time.Minute,
		detailBasicTTL:      30 * time.Minute,
		detailPriceStockTTL: time.Minute,
		preloadTTL:          2 * time.Minute,
	}
}

func ListKey(category string, page, pageSize int64) string {
	return fmt.Sprintf("product:list:%s:%d:%d", category, page, pageSize)
}

func DetailBasicKey(productID int64) string {
	return fmt.Sprintf("product:detail:basic:%d", productID)
}

func PriceInStockKey(productID, skuID int64) string {
	return fmt.Sprintf("product:detail:price_stock:%d:%d", productID, skuID)
}

func detailLoadKey(query domain.ProductQuery) string {
	return fmt.Sprintf("product:detail:load:%d:%d", query.ID, query.SKUID)
}

func (r *CacheAsideProductRepo) ListProducts(ctx context.Context, page, pageSize int64, category string) ([]domain.Product, error) {
	if page <= 3 {
		key := ListKey(category, page, pageSize)
		if data, err := r.cache.Get(ctx, key); err == nil {
			var cached []domain.Product
			if err := json.Unmarshal(data, &cached); err == nil {
				if len(cached) > 0 {
					go r.preloadProductBasics(context.Background(), cached)
				}
				return cached, nil
			}
		}

		val, err, _ := r.sf.Do(key, func() (interface{}, error) {
			products, err := r.loadProductList(ctx, page, pageSize, category)
			if err != nil {
				return nil, err
			}
			_ = r.cache.SetWithTTL(ctx, key, products, r.listCacheTTL)
			return products, nil
		})
		if err != nil {
			return nil, err
		}

		products := val.([]domain.Product)
		if len(products) > 0 {
			go r.preloadProductBasics(context.Background(), products)
		}
		return products, nil
	}

	return r.loadProductList(ctx, page, pageSize, category)
}

func (r *CacheAsideProductRepo) GetProduct(ctx context.Context, query domain.ProductQuery) (domain.Product, error) {
	cached := r.readProductCache(ctx, query)
	if cached.basicFound && cached.priceFound {
		return mergeProductPriceStock(cached.basic, cached.priceStock), nil
	}
	if ctx.Value("downgrade") == "true" {
		return domain.Product{}, errors.New("product detail cache missed while downgrade mode is enabled")
	}

	val, err, _ := r.sf.Do(detailLoadKey(query), func() (interface{}, error) {
		return r.resolveSingleProduct(ctx, query, r.readProductCache(ctx, query))
	})
	if err != nil {
		return domain.Product{}, err
	}
	return val.(domain.Product), nil
}

func (r *CacheAsideProductRepo) GetProducts(ctx context.Context, queries []domain.ProductQuery) ([]domain.Product, error) {
	if len(queries) == 0 {
		return []domain.Product{}, nil
	}
	// GetProducts serves checkout/search snapshot reads. It bypasses detail cache so
	// price and saleable status come from the current database state instead of
	// potentially stale Redis entries.
	missingBasicIDs := make([]int64, 0, len(queries))
	missingPriceQueries := make([]domain.ProductQuery, 0, len(queries))
	seenBasicIDs := make(map[int64]struct{})
	seenPriceQueries := make(map[string]struct{})

	for _, query := range queries {
		if ctx.Value("downgrade") == "true" {
			return nil, errors.New("product detail cache missed while downgrade mode is enabled")
		}
		if _, ok := seenBasicIDs[query.ID]; !ok {
			seenBasicIDs[query.ID] = struct{}{}
			missingBasicIDs = append(missingBasicIDs, query.ID)
		}
		key := detailLoadKey(query)
		if _, ok := seenPriceQueries[key]; !ok {
			seenPriceQueries[key] = struct{}{}
			missingPriceQueries = append(missingPriceQueries, query)
		}
	}

	basicProducts, err := r.loadBasicProductsByIDs(ctx, missingBasicIDs)
	if err != nil {
		return nil, err
	}
	priceStocks, err := r.loadPriceStocksByQueries(ctx, missingPriceQueries)
	if err != nil {
		return nil, err
	}

	products := make([]domain.Product, len(queries))
	for i, query := range queries {
		basic, ok := basicProducts[query.ID]
		if !ok {
			return nil, notFoundError(query)
		}

		priceStock, ok := priceStocks[detailLoadKey(query)]
		if !ok {
			return nil, notFoundError(query)
		}

		products[i] = mergeProductPriceStock(basic, priceStock)
	}

	return products, nil
}

func (r *CacheAsideProductRepo) GetProductQuotes(ctx context.Context, queries []domain.ProductQuery) ([]domain.ProductQuote, error) {
	if len(queries) == 0 {
		return []domain.ProductQuote{}, nil
	}
	if ctx.Value("downgrade") == "true" {
		return nil, errors.New("product quote cache missed while downgrade mode is enabled")
	}

	entities, err := r.dao.FindQuotes(ctx, queries)
	if err != nil {
		return nil, err
	}

	quotes := make(map[string]domain.ProductQuote, len(entities))
	for _, entity := range entities {
		query := domain.ProductQuery{ID: entity.ProductID, SKUID: entity.SKUID}
		quotes[detailLoadKey(query)] = domain.ProductQuote{
			ProductID: entity.ProductID,
			SKUID:     entity.SKUID,
			Price:     entity.Price,
			Currency:  normalizeCurrency(entity.Currency),
			InStock:   entity.InStock,
		}
	}

	result := make([]domain.ProductQuote, len(queries))
	for i, query := range queries {
		quote, ok := quotes[detailLoadKey(query)]
		if !ok {
			return nil, notFoundError(query)
		}
		result[i] = quote
	}
	return result, nil
}

func (r *CacheAsideProductRepo) CreateProduct(ctx context.Context, product domain.Product) (int64, error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	id, err := r.dao.Insert(ctx, entity)
	if err != nil {
		return 0, err
	}

	product.ID = id
	if product.SKUID > 0 {
		if err := r.dao.UpsertSKU(ctx, r.domainToSKU(product)); err != nil {
			return 0, err
		}
	}

	r.cacheProductBasicDetail(ctx, product, r.detailBasicTTL)
	if product.SKUID > 0 {
		r.cachePriceStock(ctx, domain.ProductQuery{ID: id, SKUID: product.SKUID}, productToPriceStock(product), r.detailPriceStockTTL)
	}
	return id, nil
}

func (r *CacheAsideProductRepo) UpdateProduct(ctx context.Context, product domain.Product) (int64, error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	if err := r.dao.Update(ctx, entity); err != nil {
		return 0, err
	}
	if product.SKUID > 0 {
		if err := r.dao.UpsertSKU(ctx, r.domainToSKU(product)); err != nil {
			return 0, err
		}
	}

	r.deleteProductCache(ctx, domain.ProductQuery{ID: product.ID, SKUID: product.SKUID})
	go r.delayedDelete(domain.ProductQuery{ID: product.ID, SKUID: product.SKUID})

	return product.ID, nil
}

func (r *CacheAsideProductRepo) DeleteProduct(ctx context.Context, id, userID int64) error {
	if err := r.dao.Delete(ctx, id, userID); err != nil {
		return err
	}
	if err := r.dao.DeleteSKUsByProductID(ctx, id); err != nil {
		return err
	}

	r.deleteProductBasicCache(ctx, id)
	go r.delayedDelete(domain.ProductQuery{ID: id})

	return nil
}

func (r *CacheAsideProductRepo) loadProductList(ctx context.Context, page, pageSize int64, category string) ([]domain.Product, error) {
	entities, err := r.dao.ListProducts(ctx, page, pageSize, category)
	if err != nil {
		return nil, err
	}

	products := make([]domain.Product, 0, len(entities))
	for _, entity := range entities {
		product, err := r.entityToDomain(entity)
		if err != nil {
			r.logger.Error("convert product entity failed", logger.Error(err))
			continue
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *CacheAsideProductRepo) preloadProductBasics(ctx context.Context, products []domain.Product) {
	preloadCount := 3
	if len(products) < preloadCount {
		preloadCount = len(products)
	}

	items := make([]cache.CacheItem, 0, preloadCount)
	for i := 0; i < preloadCount; i++ {
		product := products[i]
		key := DetailBasicKey(product.ID)
		if r.shouldRefresh(ctx, key, time.Minute) {
			items = append(items, cache.CacheItem{
				Key:   key,
				Value: basicProduct(product),
				TTL:   r.preloadTTL,
			})
		}
	}

	if len(items) == 0 {
		return
	}
	if err := r.cache.BatchSetWithTTL(ctx, items); err != nil {
		r.logger.Error("batch product basic preload failed", logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) shouldRefresh(ctx context.Context, key string, minTTL time.Duration) bool {
	if _, err := r.cache.Get(ctx, key); err != nil {
		return true
	}
	ttl, err := r.cache.GetTTL(ctx, key)
	return err != nil || ttl <= minTTL
}

func (r *CacheAsideProductRepo) readProductCache(ctx context.Context, query domain.ProductQuery) cachedProductParts {
	result := cachedProductParts{}

	if basicData, err := r.cache.Get(ctx, DetailBasicKey(query.ID)); err == nil {
		if err := json.Unmarshal(basicData, &result.basic); err == nil {
			result.basicFound = true
		}
	}
	if priceStockData, err := r.cache.Get(ctx, PriceInStockKey(query.ID, query.SKUID)); err == nil {
		if err := json.Unmarshal(priceStockData, &result.priceStock); err == nil {
			result.priceFound = true
		}
	}

	return result
}

func (r *CacheAsideProductRepo) resolveSingleProduct(ctx context.Context, query domain.ProductQuery, cached cachedProductParts) (domain.Product, error) {
	basic := cached.basic
	if !cached.basicFound {
		entity, err := r.dao.FindByID(ctx, query.ID)
		if err != nil {
			return domain.Product{}, err
		}
		product, err := r.entityToDomain(entity)
		if err != nil {
			return domain.Product{}, err
		}
		basic = product
		r.cacheProductBasicDetail(ctx, product, r.detailBasicTTL)
	}

	priceStock := cached.priceStock
	if !cached.priceFound {
		var err error
		priceStock, err = r.loadAndCachePriceStock(ctx, query)
		if err != nil {
			return domain.Product{}, err
		}
	}

	return mergeProductPriceStock(basic, priceStock), nil
}

func (r *CacheAsideProductRepo) loadBasicProductsByIDs(ctx context.Context, ids []int64) (map[int64]domain.Product, error) {
	if len(ids) == 0 {
		return map[int64]domain.Product{}, nil
	}

	entities, err := r.dao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	products := make(map[int64]domain.Product, len(entities))
	for _, entity := range entities {
		product, err := r.entityToDomain(entity)
		if err != nil {
			return nil, err
		}
		products[product.ID] = product
		r.cacheProductBasicDetail(ctx, product, r.detailBasicTTL)
	}
	return products, nil
}

func (r *CacheAsideProductRepo) loadPriceStocksByQueries(ctx context.Context, queries []domain.ProductQuery) (map[string]PriceInStock, error) {
	if len(queries) == 0 {
		return map[string]PriceInStock{}, nil
	}

	entities, err := r.dao.FindPriceInStocks(ctx, queries)
	if err != nil {
		return nil, err
	}

	priceStocks := make(map[string]PriceInStock, len(entities))
	for _, entity := range entities {
		priceStock := PriceInStock{
			SKUID:    entity.SKUID,
			Price:    entity.Price,
			Currency: normalizeCurrency(entity.Currency),
			InStock:  entity.InStock,
		}
		query := domain.ProductQuery{ID: entity.ProductID, SKUID: entity.SKUID}
		priceStocks[detailLoadKey(query)] = priceStock
		r.cachePriceStock(ctx, query, priceStock, r.detailPriceStockTTL)
	}
	return priceStocks, nil
}

func (r *CacheAsideProductRepo) loadAndCachePriceStock(ctx context.Context, query domain.ProductQuery) (PriceInStock, error) {
	price, currency, inStock, err := r.dao.FindPriceInStock(ctx, query.ID, query.SKUID)
	if err != nil {
		return PriceInStock{}, err
	}
	priceStock := PriceInStock{
		SKUID:    query.SKUID,
		Price:    price,
		Currency: normalizeCurrency(currency),
		InStock:  inStock,
	}
	r.cachePriceStock(ctx, query, priceStock, r.detailPriceStockTTL)
	return priceStock, nil
}

func (r *CacheAsideProductRepo) cacheProductBasicDetail(ctx context.Context, product domain.Product, ttl time.Duration) {
	key := DetailBasicKey(product.ID)
	if err := r.cache.SetWithTTL(ctx, key, basicProduct(product), ttl); err != nil {
		r.logger.Error("cache product basic detail failed", logger.Int64("product_id", product.ID), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) cachePriceStock(ctx context.Context, query domain.ProductQuery, priceStock PriceInStock, ttl time.Duration) {
	key := PriceInStockKey(query.ID, query.SKUID)
	if err := r.cache.SetWithTTL(ctx, key, priceStock, ttl); err != nil {
		r.logger.Error("cache product price and stock failed",
			logger.Int64("product_id", query.ID),
			logger.Int64("sku_id", query.SKUID),
			logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) deleteProductCache(ctx context.Context, query domain.ProductQuery) {
	r.deleteProductBasicCache(ctx, query.ID)
	if query.SKUID > 0 {
		if err := r.cache.Delete(ctx, PriceInStockKey(query.ID, query.SKUID)); err != nil {
			r.logger.Error("delete price and stock cache failed",
				logger.Int64("product_id", query.ID),
				logger.Int64("sku_id", query.SKUID),
				logger.Error(err))
		}
	}
}

func (r *CacheAsideProductRepo) deleteProductBasicCache(ctx context.Context, productID int64) {
	if err := r.cache.Delete(ctx, DetailBasicKey(productID)); err != nil {
		r.logger.Error("delete basic cache failed", logger.Int64("product_id", productID), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) delayedDelete(query domain.ProductQuery) {
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r.deleteProductCache(ctx, query)
	r.logger.Info("delayed product cache delete completed",
		logger.Int64("product_id", query.ID),
		logger.Int64("sku_id", query.SKUID))
}

func (r *CacheAsideProductRepo) domainToEntity(d domain.Product) (dao.Product, error) {
	entity := dao.Product{
		ID:           d.ID,
		Name:         d.Name,
		Price:        d.Price,
		Currency:     normalizeCurrency(d.Currency),
		InStock:      d.InStock,
		MerchantID:   d.MerchantID,
		MerchantName: sql.NullString{String: d.MerchantName, Valid: d.MerchantName != ""},
		Description:  sql.NullString{String: d.Description, Valid: d.Description != ""},
		Picture:      sql.NullString{String: d.Picture, Valid: d.Picture != ""},
	}

	slideImgs, err := json.Marshal(d.SlideImgs)
	if err != nil {
		return dao.Product{}, err
	}
	entity.SlideImgs = string(slideImgs)

	categories, err := json.Marshal(d.Categories)
	if err != nil {
		return dao.Product{}, err
	}
	entity.Categories = string(categories)

	return entity, nil
}

func (r *CacheAsideProductRepo) domainToSKU(d domain.Product) dao.ProductSKU {
	return dao.ProductSKU{
		ProductID: d.ID,
		SKUID:     d.SKUID,
		Price:     d.Price,
		Currency:  normalizeCurrency(d.Currency),
		InStock:   d.InStock,
	}
}

func (r *CacheAsideProductRepo) entityToDomain(e dao.Product) (domain.Product, error) {
	product := domain.Product{
		ID:         e.ID,
		Name:       e.Name,
		Price:      e.Price,
		Currency:   normalizeCurrency(e.Currency),
		InStock:    e.InStock,
		MerchantID: e.MerchantID,
	}

	if e.Description.Valid {
		product.Description = e.Description.String
	}
	if e.Picture.Valid {
		product.Picture = e.Picture.String
	}
	if e.MerchantName.Valid {
		product.MerchantName = e.MerchantName.String
	}

	if e.SlideImgs != "" && e.SlideImgs != "[]" {
		if err := json.Unmarshal([]byte(e.SlideImgs), &product.SlideImgs); err != nil {
			return domain.Product{}, err
		}
	}
	if product.SlideImgs == nil {
		product.SlideImgs = []string{}
	}

	if e.Categories != "" && e.Categories != "[]" {
		if err := json.Unmarshal([]byte(e.Categories), &product.Categories); err != nil {
			return domain.Product{}, err
		}
	}
	if product.Categories == nil {
		product.Categories = []string{}
	}

	return product, nil
}

func mergeProductPriceStock(product domain.Product, priceStock PriceInStock) domain.Product {
	product.SKUID = priceStock.SKUID
	product.Price = priceStock.Price
	product.Currency = normalizeCurrency(priceStock.Currency)
	product.InStock = priceStock.InStock
	return product
}

func basicProduct(product domain.Product) domain.Product {
	return domain.Product{
		ID:           product.ID,
		Name:         product.Name,
		Description:  product.Description,
		Picture:      product.Picture,
		SlideImgs:    product.SlideImgs,
		Categories:   product.Categories,
		MerchantID:   product.MerchantID,
		MerchantName: product.MerchantName,
	}
}

func productToPriceStock(product domain.Product) PriceInStock {
	return PriceInStock{
		SKUID:    product.SKUID,
		Price:    product.Price,
		Currency: normalizeCurrency(product.Currency),
		InStock:  product.InStock,
	}
}

func normalizeCurrency(currency string) string {
	if currency == "" {
		return "CNY"
	}
	return currency
}

func notFoundError(query domain.ProductQuery) error {
	return fmt.Errorf("%w: product_id=%d sku_id=%d", dao.ErrDataNotFound, query.ID, query.SKUID)
}
