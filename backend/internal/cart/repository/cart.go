package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/cart/domain"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/cache"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/dao"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CartRepository interface {
	AddItems(ctx context.Context, userID int64, items []domain.CartItem) error
	DeleteItems(ctx context.Context, userID int64, productIDs []int64) error
	GetCart(ctx context.Context, userID int64) (domain.Cart, error)
	EmptyCart(ctx context.Context, userID int64) error
	ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error
	IncrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error)
	DecrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error)
}

type cachedCartRepository struct {
	cache  cache.CartCache
	dao    dao.CartDAO
	logger logger.LoggerV1
}

func NewCachedCartRepository(cache cache.CartCache, dao dao.CartDAO, logger logger.LoggerV1) CartRepository {
	return &cachedCartRepository{
		cache:  cache,
		dao:    dao,
		logger: logger,
	}
}

func (r *cachedCartRepository) cartKey(userID int64) string {
	return fmt.Sprintf("cart:%d", userID)
}

func (r *cachedCartRepository) productField(productID int64) string {
	return strconv.FormatInt(productID, 10)
}

// 涓氬姟鎬ц兘姣旀暟鎹竴鑷存€э紝姝ｇ‘鎬ч噸瑕侊紝涔熷氨鏄蹇嶄竴浜涙暟鎹敊璇紝閫夋嫨 write-behind
func (r *cachedCartRepository) AddItems(ctx context.Context, userID int64, items []domain.CartItem) error {
	key := r.cartKey(userID)
	fields := make([]string, len(items))
	for i, item := range items {
		fields[i] = r.productField(item.ProductID)
	}
	if _, err := r.cache.HIncrBy(ctx, key, fields...); err != nil {
		return err
	}
	productIDs := make([]int64, len(items))
	for i, item := range items {
		productIDs[i] = item.ProductID
	}
	// 鍗曟潯 INSERT ... ON CONFLICT 鎵归噺鍐?MySQL
	go r.asyncUpsertIncrementBatch(userID, productIDs)
	return nil
}

func (r *cachedCartRepository) DeleteItems(ctx context.Context, userID int64, productIDs []int64) error {
	key := r.cartKey(userID)
	fields := make([]string, len(productIDs))
	for i, pid := range productIDs {
		fields[i] = r.productField(pid)
	}
	if err := r.cache.HDel(ctx, key, fields...); err != nil {
		return err
	}
	go r.asyncDeleteByProductIDs(userID, productIDs)
	return nil
}

func (r *cachedCartRepository) GetCart(ctx context.Context, userID int64) (domain.Cart, error) {
	key := r.cartKey(userID)

	result, err := r.cache.HGetAll(ctx, key)
	if err == nil && len(result) > 0 {
		return r.mapToCart(userID, result), nil
	}

	daoItems, err := r.dao.FindCartByUserID(ctx, userID)
	if err != nil {
		return domain.Cart{}, err
	}
	if len(daoItems) == 0 {
		return domain.Cart{UserID: userID, Items: []domain.CartItem{}}, nil
	}

	go r.asyncWriteBackToRedis(userID, daoItems)

	items := make([]domain.CartItem, 0, len(daoItems))
	for _, daoItem := range daoItems {
		items = append(items, domain.CartItem{
			ProductID: daoItem.ProductID,
			Quantity:  daoItem.Quantity,
		})
	}

	return domain.Cart{
		UserID: userID,
		Items:  items,
	}, nil
}

func (r *cachedCartRepository) EmptyCart(ctx context.Context, userID int64) error {
	key := r.cartKey(userID)

	err := r.cache.Del(ctx, key)
	if err != nil {
		return err
	}

	go r.asyncEmptyCart(userID)

	return nil
}

func (r *cachedCartRepository) ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error {
	key := r.cartKey(userID)
	field := r.productField(item.ProductID)

	err := r.cache.HSet(ctx, key, field, item.Quantity)
	if err != nil {
		return err
	}

	go r.asyncWriteToMySQL(userID, item)

	return nil
}

func (r *cachedCartRepository) IncrementQty(ctx context.Context, userID, productID int64) (int64, error) {
	key := r.cartKey(userID)
	field := r.productField(productID)

	qtys, err := r.cache.HIncrBy(ctx, key, field)
	if err != nil {
		return 0, err
	}
	newQty := qtys[0]
	if err != nil {
		return 0, err
	}

	go r.asyncIncrementToMySQL(userID, productID)

	return newQty, nil
}

func (r *cachedCartRepository) DecrementQty(ctx context.Context, userID, productID int64) (int64, error) {
	key := r.cartKey(userID)
	field := r.productField(productID)

	newQty, err := r.cache.DecrementIfGreaterThanOne(ctx, key, field)
	if err != nil {
		return 0, err
	}

	go r.asyncDecrementToMySQL(userID, productID)

	return newQty, nil
}

// ----------------- 杈呭姪鏂规硶 ---------------------

func (r *cachedCartRepository) mapToCart(userID int64, result map[string]string) domain.Cart {
	items := make([]domain.CartItem, 0, len(result))
	for productIDStr, quantityStr := range result {
		productID, _ := strconv.ParseInt(productIDStr, 10, 64)
		quantity, _ := strconv.ParseInt(quantityStr, 10, 64)
		items = append(items, domain.CartItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	return domain.Cart{
		UserID: userID,
		Items:  items,
	}
}

func (r *cachedCartRepository) asyncWriteToMySQL(userID int64, item domain.CartItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	daoItem := dao.CartItem{
		UserID:    userID,
		ProductID: item.ProductID,
		Quantity:  item.Quantity,
	}

	err := r.dao.Upsert(ctx, daoItem)
	if err != nil {
		r.logger.Error("寮傛鍐?MySQL 澶辫触", logger.Int64("user_id", userID), logger.Int64("product_id", item.ProductID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncUpsertIncrementBatch(userID int64, productIDs []int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.dao.UpsertIncrementBatch(ctx, userID, productIDs); err != nil {
		r.logger.Error("寮傛鎵归噺绱姞 MySQL 澶辫触", logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncDeleteByProductIDs(userID int64, productIDs []int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.dao.DeleteByProductIDs(ctx, userID, productIDs); err != nil {
		r.logger.Error("寮傛鎵归噺鍒犻櫎 MySQL 澶辫触", logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncIncrementToMySQL(userID, productID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.dao.IncrementQuantity(ctx, userID, productID)
	if err != nil {
		r.logger.Error("寮傛澧炲姞 MySQL 澶辫触", logger.Int64("user_id", userID), logger.Int64("product_id", productID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncDecrementToMySQL(userID, productID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.dao.DecrementQuantity(ctx, userID, productID)
	if err != nil {
		r.logger.Error("寮傛鍑忓皯 MySQL 澶辫触", logger.Int64("user_id", userID), logger.Int64("product_id", productID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncEmptyCart(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.dao.DeleteByUserID(ctx, userID)
	if err != nil {
		r.logger.Error("寮傛娓呯┖璐墿杞﹀け璐?, logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncWriteBackToRedis(userID int64, daoItems []dao.CartItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := r.cartKey(userID)
	fieldValues := make(map[string]int64, len(daoItems))
	for _, item := range daoItems {
		fieldValues[r.productField(item.ProductID)] = item.Quantity
	}
	if err := r.cache.HSetBatch(ctx, key, fieldValues); err != nil {
		r.logger.Error("鎵归噺鍥炲啓 Redis 澶辫触", logger.Int64("user_id", userID), logger.Error(err))
	}
}


