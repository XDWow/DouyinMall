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
// Cache-Aside 濡€崇础鐎圭偟骞囬敍鍦? 閻楀牊婀伴敍?
// =============================================================================
//
// 閺嬭埖鐎悧鍦仯閿?
//   - 娑撴艾濮熸禒锝囩垳閻╁瓨甯寸粻锛勬倞缂傛挸鐡ㄩ敍鍫ｎ嚢閸愭瑥鍨归敍?
//   - 閺囧瓨鏌?閸掔娀娅庨弮鏈靛瘜閸斻劌銇戦弫鍫㈢处鐎涙﹫绱欏鎯扮箿閸欏苯鍨归敍?
//   - 闁倻鏁ゆ禍搴″礋 Redis閵嗕焦妫?MQ 閻ㄥ嫮鐣濋崡鏇炴簚閺?
//
// 缂傛挸鐡ㄧ粵鏍殣閿?
//   - 閸掓銆冪紓鎾崇摠閿涙氨鐓?TTL閿?閸掑棝鎸撻敍澶涚礉閻戭厾鍋ｉ弫鐗堝祦 singleflight 闂冩彃鍤粚?
//   - 鐠囷附鍎忕紓鎾崇摠閿涙艾鐔€閺堫兛淇婇幁?30 閸掑棝鎸撻敍灞肩幆閺?鎼存挸鐡ㄩ悩鑸碘偓?1 閸掑棝鎸撻敍鍫濆瀻缁傝鐡ㄩ崒顭掔礆
//   - 妫板嫮鍎圭紓鎾崇摠閿? 閸掑棝鎸撻敍鍫ヮ暕濞村鈧嗗窛閿?
//
// 娑撯偓閼峰瓨鈧傜箽闂呮粣绱?
//   - 瀵ゆ儼绻滈崣灞藉灩閿涙俺袙閸愬啿鑻熼崣鎴ｎ嚢閸愭瑥顕遍懛瀵告畱閼村繒绱︾€涙﹢妫舵０姗堢礄娑撹绮犲鎯扮箿娴兼艾濮為崜褝绱?
//   - 閸掓銆冪紓鎾崇摠娑撳秳瀵岄崝銊ュ灩闂勩倧绱濇笟婵婄閻?TTL 閼奉亞鍔ф潻鍥ㄦ埂閿涘牆褰夐崠鏍暥缁讳緤绱濋崚鐘绘珟閹存劖婀版姗堢礆
//   - 鐠囷附鍎忕紓鎾崇摠娑撹濮╅崚鐘绘珟 + TTL 閸忔粌绨抽敍鍫濆灩闂勩倕銇戠拹銉︽ TTL 娣囨繆鐦夐張鈧紒鍫滅閼疯揪绱?
//
// 缂傝櫣鍋ｉ敍?
//   - 娴狅絿鐖滄笟闈涘弳閿涙碍鐦℃径鍕晸閹垮秳缍旈柈鍊燁洣缁狅紕鎮婄紓鎾崇摠
//   - 閹碘晛鐫嶉幀褍妯婇敍姘煀婢х偟绱︾€涙﹫绱欐俊?ES閵嗕礁顦?Redis閿涘娓剁憰浣锋叏閺€閫涘敩閻?
//   - 閸欘垶娼幀褍鎬ラ敍姘灩缂傛挸鐡ㄦ径杈Е娴兼艾顕遍懛缈犵瑝娑撯偓閼?
//
// 娴滎喚鍋ｉ敍?
//	閸掑棛顬囩紓鎾崇摠	閸╃儤婀版穱鈩冧紖 30min閿涘奔鐜弽?鎼存挸鐡ㄩ悩鑸碘偓?1min閿涘瞼瀚粩?TTL
//	缁儳鍣弻銉嚄	娴犻攱鐗?鎼存挸鐡ㄩ悩鑸碘偓?miss 閺冭泛褰ч弻?SELECT price, in_stock閿涘苯鍣虹亸鎴犵秹缂佹粈绱舵潏?
//	singleflight	閻戭厾鍋ｉ弫鐗堝祦閿涘牆澧?妞ょ绱氶梼鑼处鐎涙ê鍤粚?
//	缂傛挸鐡ㄦ０鍕劰	閸掓銆冮弻銉嚄閸氬酣顣╅悜顓炲3娑擃亜鏅㈤崫浣筋嚊閹?
//	瀵ゆ儼绻滈崣灞藉灩	閺囧瓨鏌婇崥搴ｇ彌閸楀啿鍨?+ 瀵ゆ儼绻?缁夋帒鍟€閸掔媴绱濈憴锝呭枀楠炶泛褰傞懘蹇曠处鐎?
//	缁屽潡鈧繘妲婚幎?缁岀儤鏆熺紒鍕瘍缂傛挸鐡?
//	闂勫秶楠囩粵鏍殣	ctx.Value("downgrade") 閹貉冨煑

// =============================================================================

type CacheAsideProductRepo struct {
	dao    dao.ProductDao
	cache  cache.ProductCache
	logger logger.LoggerV1

	// TTL 闁板秶鐤?
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

		listCacheTTL:        3 * time.Minute,  // 閸掓銆冮敍?閸掑棝鎸撻敍鍫滅瑝娑撹濮╅崚鐘绘珟閿涘奔绶风挧鏍殰閻掓儼绻冮張鐕傜礆
		detailBasicTTL:      30 * time.Minute, // 閸╃儤婀版穱鈩冧紖閿?0閸掑棝鎸撻敍鍫滃瘜閸斻劌鍨归梽?+ TTL 閸忔粌绨抽敍?
		detailPriceStockTTL: 1 * time.Minute,  // 娴犻攱鐗?鎼存挸鐡ㄩ悩鑸碘偓渚婄窗1閸掑棝鎸撻敍鍫滃瘜閸斻劌鍨归梽?+ TTL 閸忔粌绨抽敍灞炬付婢?1 閸掑棝鎸撻幁銏狀槻娑撯偓閼疯揪绱?
		preloadTTL:          2 * time.Minute,  // 妫板嫮鍎归敍?閸掑棝鎸撻敍鍫ヮ暕濞村銇戠拹銉ユ彥闁喕绻冮張鐕傜礆
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
	// 閻戭厾鍋ｉ弫鐗堝祦閿涘牆澧?妞ょ绱氭担璺ㄦ暏 singleflight 闂冨弶顒涚紓鎾崇摠閸戣崵鈹?
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

		// singleflight閿涙艾鎮庨獮鍓佹祲閸?key 閻ㄥ嫬鑻熼崣鎴ｎ嚞濮瑰偊绱濋崣顏呮箒娑撯偓娑擃亣顕Ч鍌涚叀閺佺増宓佹惔?
		val, err, _ := r.sf.Do(key, func() (interface{}, error) {
			ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
			if err != nil {
				return nil, err
			}

			res := make([]domain.Product, 0, len(ps))
			for _, p := range ps {
				domainProduct, err := r.entityToDomain(p)
				if err != nil {
					r.logger.Error("鏉烆剚宕查崯鍡楁惂閺佺増宓佹径杈Е", logger.Error(err))
					continue
				}
				res = append(res, domainProduct)
			}

			// 缂傛挸鐡ㄧ紒鎾寸亯閿涘牏鈹栭弫鎵矋娑旂喓绱︾€涙﹫绱濋梼鍙夘剾缁屽潡鈧骏绱?
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

	// 闂堢偟鍎归悙瑙勬殶閹诡噯绱濋惄瀛樺复閺屻儲鏆熼幑顔肩氨
	ps, err := r.dao.ListProducts(ctx, page, pageSize, category)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		domainProduct, err := r.entityToDomain(p)
		if err != nil {
			r.logger.Error("鏉烆剚宕查崯鍡楁惂閺佺増宓佹径杈Е", logger.Error(err))
			continue
		}
		products = append(products, domainProduct)
	}
	return products, nil
}

// 閻劍鍩涘ù蹇氼潔閸掓銆冮崥搴°亣濮掑倻宸奸悙鐟板毊閸撳秴鍤戞稉顏勬櫌閸濅緤绱濋幓鎰缂傛挸鐡ㄩ崙蹇撶毌瀵ゆ儼绻?
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

		// 濡偓閺屻儱鐔€閺堫兛淇婇幁顖滅处鐎涙﹫绱濇稉宥呯摠閸︺劍鍨ㄨ箛顐ョ箖閺堢喎鍨０鍕劰
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

		// 濡偓閺屻儰鐜弽?鎼存挸鐡ㄩ悩鑸碘偓浣虹处鐎涙﹫绱濇稉宥呯摠閸︺劍鍨ㄨ箛顐ョ箖閺堢喎鍨０鍕劰
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
			r.logger.Error("batch cache preload failed", logger.Error(err))
		}
	}
}

func (r *CacheAsideProductRepo) GetProduct(ctx context.Context, id int64) (product domain.Product, err error) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	// 1. 鐏忔繆鐦禒搴ｇ处鐎涙骞忛崣?
	basicData, basicErr := r.cache.Get(ctx, basicKey)
	priceInStockData, priceInStockErr := r.cache.Get(ctx, priceInStockKey)

	// 閹懎鍠?閿涙岸鍏橀崨鎴掕厬閿涘瞼娲块幒銉ㄧ箲閸?
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

	// 閹懎鍠?閿涙艾鐔€閺堫兛淇婇幁顖氭嚒娑擃叏绱濇禒閿嬬壐/鎼存挸鐡ㄩ悩鑸碘偓浣规弓閸涙垝鑵戦敍鍫濆瀻缁傝鐡ㄩ崒銊ф畱娴兼ê濞嶉敍?
	// 娴兼ê瀵查悙鐧哥窗閸欘亝鐓?price, in_stock 娑撱倓閲滅€涙顔岄敍灞肩瑝閺屻儱鐣弫鏉戞櫌閸?
	if basicErr == nil && priceInStockErr != nil {
		if err := json.Unmarshal(basicData, &product); err == nil {
			price, inStock, err := r.dao.FindPriceInStock(ctx, id)
			if err != nil {
				r.logger.Error("閺屻儴顕楁禒閿嬬壐/鎼存挸鐡ㄩ悩鑸碘偓浣搞亼鐠愩儻绱濇担璺ㄦ暏闂勫秶楠囬弫鐗堝祦",
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

	// 閹懎鍠?閿涙岸鍏橀張顏勬嚒娑擃叏绱濋幋鏍х唨閺堫兛淇婇幁顖涚梾娑擃叏绱濋崣顏冭厬娴滃棔鐜弽纭风礉閻╁瓨甯撮弻銉ョ氨
	// 闂勫秶楠?
	if ctx.Value("downgrade") == "true" {
		return domain.Product{}, errors.New("闂勫秶楠囩粵鏍殣閿涙氨绱︾€涙ɑ婀崨鎴掕厬閿涘矁鐑︽潻鍥ㄦ殶閹诡喖绨遍弻銉嚄")
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
	Price   int64 `json:"price"`    // 閸楁洑缍呴敍姘瀻
	InStock bool  `json:"in_stock"` // 閺勵垰鎯侀張澶庢彛
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
		r.logger.Error("cache product basic detail failed", logger.Int64("product_id", product.ID), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) cachePriceInStock(ctx context.Context, id int64, price int64, inStock bool, ttl time.Duration) {
	key := PriceInStockKey(id)
	ps := PriceInStock{Price: price, InStock: inStock}
	if err := r.cache.SetWithTTL(ctx, key, ps, ttl); err != nil {
		r.logger.Error("cache product price and stock failed", logger.Int64("product_id", id), logger.Error(err))
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

// 瀵ゆ儼绻滈崣灞藉灩閿涘矁袙閸愬啿鑻熼崣鎴ｎ嚢閸愭瑥顕遍懛瀵告畱閼村繒绱︾€涙﹢妫舵０?
func (r *CacheAsideProductRepo) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	entity, err := r.domainToEntity(product)
	if err != nil {
		return 0, err
	}

	// 1. 閺囧瓨鏌婇弫鐗堝祦鎼?
	err = r.dao.Update(ctx, entity)
	if err != nil {
		return 0, err
	}

	// 2. 缁斿宓嗛崚鐘绘珟缂傛挸鐡ㄩ敍鍫㈩儑娑撯偓濞嗏槄绱氶敍姘愁唨閸氬海鐢荤拠閿嬬湴閺屻儱绨遍懢宄板絿閺傜増鏆熼幑?
	r.deleteProductCache(ctx, product.ID)

	// 3. 瀵ゆ儼绻滈崥搴″晙閸掔娀娅庨敍鍫㈩儑娴滃本顐奸敍澶涚窗濞撳懘娅庨獮璺哄絺鐠囪鍟撴禍褏鏁撻惃鍕壈缂傛挸鐡?
	// 閸︾儤娅欓敍姘愁嚞濮?閺屻儳绱︾€涙コiss,閺屻儲鏆熼幑顔肩氨閿涘牊妫敍?鐠囬攱鐪?閺囧瓨鏌婇弫鐗堝祦鎼存搫绱欓弬甯礆閿涘矁顕Ч?閸掔姷绱︾€涙﹫绱濈拠閿嬬湴1閸愭瑦妫弫鐗堝祦閸掓壆绱︾€?
	// 缁楊兛绨╁▎鈥虫鏉╃喎鍨归梽銈呭讲娴犮儲绔荤粚楦跨箹娑擃亝妫弫鐗堝祦
	go r.delayedDelete(product.ID)

	return product.ID, nil
}

// 鏉╂瑩鍣烽悽銊ф畱閺勵垵钂嬮崚鐘绘珟閿涙艾鐤勯梽鍛瘍閺勵垱娲块弬鏉跨摟濞堢绱濇稉鈧弽鐤洣瀵ゆ儼绻滈崣灞藉灩
func (r *CacheAsideProductRepo) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	err = r.dao.Delete(ctx, id, userID)
	if err != nil {
		return err
	}

	r.deleteProductCache(ctx, id)

	go r.delayedDelete(id)

	return nil
}

// deleteProductCache 閸掔娀娅庨崯鍡楁惂鐠囷附鍎忕紓鎾崇摠閿涘奔绗夐崚鐘绘珟閸掓銆冪紓鎾崇摠
// 閸樼喎娲滈敍?
//  1. 閸掓銆冪紓鎾崇摠閸欐ê瀵叉０鎴犵畳閿涘牊鏌婂?閺囧瓨鏌?閸掔娀娅庨崯鍡楁惂闁垝绱拌ぐ鍗炴惙婢舵矮閲滈崚妤勩€冩い纰夌礆閿涘苯绱戦柨鈧径?
//  2. 閸掓銆冮惄绋款嚠閸熷棗鎼х拠锔藉剰鐎圭偞妞傞幀褑顩﹀Ч鍌濈窛娴ｅ函绱濋柅姘崇箖閻?TTL閿?閸掑棝鎸撻敍澶庡殰閻掓儼绻冮張鐔峰祮閸欘垽绱濇稉璇插З閸掔姵鏁归惄濠傜毈
func (r *CacheAsideProductRepo) deleteProductCache(ctx context.Context, id int64) {
	basicKey := DetailBasicKey(id)
	priceInStockKey := PriceInStockKey(id)

	if err := r.cache.Delete(ctx, basicKey); err != nil {
		r.logger.Error("delete basic cache failed", logger.Int64("product_id", id), logger.Error(err))
	}
	if err := r.cache.Delete(ctx, priceInStockKey); err != nil {
		r.logger.Error("delete price and stock cache failed", logger.Int64("product_id", id), logger.Error(err))
	}
}

func (r *CacheAsideProductRepo) delayedDelete(id int64) {
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r.deleteProductCache(ctx, id)
	r.logger.Info("delayed cache delete completed", logger.Int64("product_id", id))
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
