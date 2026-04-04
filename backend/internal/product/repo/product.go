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
// Cache-Aside 妯″紡瀹炵幇锛圴1 鐗堟湰锛?
// =============================================================================
//
// 鏋舵瀯鐗圭偣锛?
//   - 涓氬姟浠ｇ爜鐩存帴绠＄悊缂撳瓨锛堣鍐欏垹锛?
//   - 鏇存柊/鍒犻櫎鏃朵富鍔ㄥけ鏁堢紦瀛橈紙寤惰繜鍙屽垹锛?
//   - 閫傜敤浜庡崟 Redis銆佹棤 MQ 鐨勭畝鍗曞満鏅?
//
// 缂撳瓨绛栫暐锛?
//   - 鍒楄〃缂撳瓨锛氱煭 TTL锛?鍒嗛挓锛夛紝鐑偣鏁版嵁 singleflight 闃插嚮绌?
//   - 璇︽儏缂撳瓨锛氬熀鏈俊鎭?30 鍒嗛挓锛屼环鏍?搴撳瓨鐘舵€?1 鍒嗛挓锛堝垎绂诲瓨鍌級
//   - 棰勭儹缂撳瓨锛? 鍒嗛挓锛堥娴嬫€ц川锛?
//
// 涓€鑷存€т繚闅滐細
//   - 寤惰繜鍙屽垹锛氳В鍐冲苟鍙戣鍐欏鑷寸殑鑴忕紦瀛橀棶棰橈紙涓讳粠寤惰繜浼氬姞鍓э級
//   - 鍒楄〃缂撳瓨涓嶄富鍔ㄥ垹闄わ紝渚濊禆鐭?TTL 鑷劧杩囨湡锛堝彉鍖栭绻侊紝鍒犻櫎鎴愭湰楂橈級
//   - 璇︽儏缂撳瓨涓诲姩鍒犻櫎 + TTL 鍏滃簳锛堝垹闄ゅけ璐ユ椂 TTL 淇濊瘉鏈€缁堜竴鑷达級
//
// 缂虹偣锛?
//   - 浠ｇ爜渚靛叆锛氭瘡澶勫啓鎿嶄綔閮借绠＄悊缂撳瓨
//   - 鎵╁睍鎬у樊锛氭柊澧炵紦瀛橈紙濡?ES銆佸 Redis锛夐渶瑕佷慨鏀逛唬鐮?
//   - 鍙潬鎬у急锛氬垹缂撳瓨澶辫触浼氬鑷翠笉涓€鑷?
//
// 浜偣锛?
//	鍒嗙缂撳瓨	鍩烘湰淇℃伅 30min锛屼环鏍?搴撳瓨鐘舵€?1min锛岀嫭绔?TTL
//	绮惧噯鏌ヨ	浠锋牸/搴撳瓨鐘舵€?miss 鏃跺彧鏌?SELECT price, in_stock锛屽噺灏戠綉缁滀紶杈?
//	singleflight	鐑偣鏁版嵁锛堝墠3椤碉級闃茬紦瀛樺嚮绌?
//	缂撳瓨棰勭儹	鍒楄〃鏌ヨ鍚庨鐑墠3涓晢鍝佽鎯?
//	寤惰繜鍙屽垹	鏇存柊鍚庣珛鍗冲垹 + 寤惰繜1绉掑啀鍒狅紝瑙ｅ喅骞跺彂鑴忕紦瀛?
//	绌块€忛槻鎶?绌烘暟缁勪篃缂撳瓨
//	闄嶇骇绛栫暐	ctx.Value("downgrade") 鎺у埗

// =============================================================================

type CacheAsideProductRepo struct {
	dao    dao.ProductDao
	cache  cache.ProductCache
	logger logger.LoggerV1

	// TTL 閰嶇疆
	listCacheTTL        time.Duration
	detailBasicTTL      time.Duration
	detailPriceStockTTL time.Duration
	preloadTTL          time.Duration

	sf singleflight.Group
}

func NewCachedProductRepo(dao dao.ProductDao, cache cache.ProductCache, l logger.LoggerV1) ProductRepo {
	return &CacheAsideProductRepo{
		dao:    dao,
		cache:  cache,
		logger: l,

		listCacheTTL:        3 * time.Minute,  // 鍒楄〃锛?鍒嗛挓锛堜笉涓诲姩鍒犻櫎锛屼緷璧栬嚜鐒惰繃鏈燂級
		detailBasicTTL:      30 * time.Minute, // 鍩烘湰淇℃伅锛?0鍒嗛挓锛堜富鍔ㄥ垹闄?+ TTL 鍏滃簳锛?
		detailPriceStockTTL: 1 * time.Minute,  // 浠锋牸/搴撳瓨鐘舵€侊細1鍒嗛挓锛堜富鍔ㄥ垹闄?+ TTL 鍏滃簳锛屾渶澶?1 鍒嗛挓鎭㈠涓€鑷达級
		preloadTTL:          2 * time.Minute,  // 棰勭儹锛?鍒嗛挓锛堥娴嬪け璐ュ揩閫熻繃鏈燂級
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
	// 鐑偣鏁版嵁锛堝墠3椤碉級浣跨敤 singleflight 闃叉缂撳瓨鍑荤┛
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

		// singleflight锛氬悎骞剁浉鍚?key 鐨勫苟鍙戣姹傦紝鍙湁涓€涓姹傛煡鏁版嵁搴?
		val, err, _ := r.sf.Do(key, func() (interface{}, error) {
			ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
			if err != nil {
				return nil, err
			}

			res := make([]domain.Product, 0, len(ps))
			for _, p := range ps {
				domainProduct, err := r.entityToDomain(p)
				if err != nil {
					r.logger.Error("杞崲鍟嗗搧鏁版嵁澶辫触", logger.Error(err))
					continue
				}
				res = append(res, domainProduct)
			}

			// 缂撳瓨缁撴灉锛堢┖鏁扮粍涔熺紦瀛橈紝闃叉绌块€忥級
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

	// 闈炵儹鐐规暟鎹紝鐩存帴鏌ユ暟鎹簱
	ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		domainProduct, err := r.entityToDomain(p)
		if err != nil {
			r.logger.Error("杞崲鍟嗗搧鏁版嵁澶辫触", logger.Error(err))
			continue
		}
		products = append(products, domainProduct)
	}
	return products, nil
}

// 鐢ㄦ埛娴忚鍒楄〃鍚庡ぇ姒傜巼鐐瑰嚮鍓嶅嚑涓晢鍝侊紝鎻愬墠缂撳瓨鍑忓皯寤惰繜
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

		// 妫€鏌ュ熀鏈俊鎭紦瀛橈紝涓嶅瓨鍦ㄦ垨蹇繃鏈熷垯棰勭儹
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

		// 妫€鏌ヤ环鏍?搴撳瓨鐘舵€佺紦瀛橈紝涓嶅瓨鍦ㄦ垨蹇繃鏈熷垯棰勭儹
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
			r.logger.Error("鎵归噺棰勭紦瀛樺晢鍝佸け璐?, logger.Error(err))
		}
	}
}

func (r *CacheAsideProductRepo) GetProduct(ctx context.Context, id int64) (product domain.Product, err error) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	// 1. 灏濊瘯浠庣紦瀛樿幏鍙?
	basicData, basicErr := r.cache.Get(ctx, basicKey)
	priceInStockData, priceInStockErr := r.cache.Get(ctx, priceInStockKey)

	// 鎯呭喌1锛氶兘鍛戒腑锛岀洿鎺ヨ繑鍥?
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

	// 鎯呭喌2锛氬熀鏈俊鎭懡涓紝浠锋牸/搴撳瓨鐘舵€佹湭鍛戒腑锛堝垎绂诲瓨鍌ㄧ殑浼樺娍锛?
	// 浼樺寲鐐癸細鍙煡 price, in_stock 涓や釜瀛楁锛屼笉鏌ュ畬鏁村晢鍝?
	if basicErr == nil && priceInStockErr != nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			price, inStock, err := r.dao.FindPriceInStock(ctx, id)
			if err != nil {
				r.logger.Error("鏌ヨ浠锋牸/搴撳瓨鐘舵€佸け璐ワ紝浣跨敤闄嶇骇鏁版嵁",
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

	// 鎯呭喌3锛氶兘鏈懡涓紝鎴栧熀鏈俊鎭病涓紝鍙腑浜嗕环鏍硷紝鐩存帴鏌ュ簱
	// 闄嶇骇
	if ctx.Value("downgrade") == "true" {
		return domain.Product{}, errors.New("闄嶇骇绛栫暐锛氱紦瀛樻湭鍛戒腑锛岃烦杩囨暟鎹簱鏌ヨ")
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
	Price   int64 `json:"price"`    // 鍗曚綅锛氬垎
	InStock bool  `json:"in_stock"` // 鏄惁鏈夎揣
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
		r.logger.Error("缂撳瓨鍟嗗搧鍩烘湰淇℃伅澶辫触", logger.Int64("product_id", product.ID), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) cachePriceInStock(ctx context.Context, id int64, price int64, inStock bool, ttl time.Duration) {
	key := PriceInStockKey(id)
	ps := PriceInStock{Price: price, InStock: inStock}
	if err := r.cache.SetWithTTL(ctx, key, ps, ttl); err != nil {
		r.logger.Error("缂撳瓨浠锋牸/搴撳瓨鐘舵€佸け璐?, logger.Int64("product_id", id), logger.Error(err))
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

// 寤惰繜鍙屽垹锛岃В鍐冲苟鍙戣鍐欏鑷寸殑鑴忕紦瀛橀棶棰?
func (r *CacheAsideProductRepo) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	// 1. 鏇存柊鏁版嵁搴?
	err = r.dao.Update(ctx, entity)
	if err != nil {
		return 0, err
	}

	// 2. 绔嬪嵆鍒犻櫎缂撳瓨锛堢涓€娆★級锛氳鍚庣画璇锋眰鏌ュ簱鑾峰彇鏂版暟鎹?
	r.deleteProductCache(ctx, product.ID)

	// 3. 寤惰繜鍚庡啀鍒犻櫎锛堢浜屾锛夛細娓呴櫎骞跺彂璇诲啓浜х敓鐨勮剰缂撳瓨
	// 鍦烘櫙锛氳姹?鏌ョ紦瀛楳iss,鏌ユ暟鎹簱锛堟棫锛?璇锋眰2鏇存柊鏁版嵁搴擄紙鏂帮級锛岃姹?鍒犵紦瀛橈紝璇锋眰1鍐欐棫鏁版嵁鍒扮紦瀛?
	// 绗簩娆″欢杩熷垹闄ゅ彲浠ユ竻绌鸿繖涓棫鏁版嵁
	go r.delayedDelete(product.ID)

	return product.ID, nil
}

// 杩欓噷鐢ㄧ殑鏄蒋鍒犻櫎锛氬疄闄呬篃鏄洿鏂板瓧娈碉紝涓€鏍疯寤惰繜鍙屽垹
func (r *CacheAsideProductRepo) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	err = r.dao.Delete(ctx, id, userID)
	if err != nil {
		return err
	}

	r.deleteProductCache(ctx, id)

	go r.delayedDelete(id)

	return nil
}

// deleteProductCache 鍒犻櫎鍟嗗搧璇︽儏缂撳瓨锛屼笉鍒犻櫎鍒楄〃缂撳瓨
// 鍘熷洜锛?
//  1. 鍒楄〃缂撳瓨鍙樺寲棰戠箒锛堟柊寤?鏇存柊/鍒犻櫎鍟嗗搧閮戒細褰卞搷澶氫釜鍒楄〃椤碉級锛屽紑閿€澶?
//  2. 鍒楄〃鐩稿鍟嗗搧璇︽儏瀹炴椂鎬ц姹傝緝浣庯紝閫氳繃鐭?TTL锛?鍒嗛挓锛夎嚜鐒惰繃鏈熷嵆鍙紝涓诲姩鍒犳敹鐩婂皬
func (r *CacheAsideProductRepo) deleteProductCache(ctx context.Context, id int64) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	if err := r.cache.Delete(ctx, basicKey); err != nil {
		r.logger.Error("鍒犻櫎鍩烘湰淇℃伅缂撳瓨澶辫触", logger.Int64("product_id", id), logger.Error(err))
	}
	if err := r.cache.Delete(ctx, priceInStockKey); err != nil {
		r.logger.Error("鍒犻櫎浠锋牸/搴撳瓨鐘舵€佺紦瀛樺け璐?, logger.Int64("product_id", id), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) delayedDelete(id int64) {
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r.deleteProductCache(ctx, id)
	r.logger.Info("寤惰繜鍙屽垹瀹屾垚", logger.Int64("product_id", id))
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


