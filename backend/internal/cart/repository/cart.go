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
	DeleteItems(ctx context.Context, userID int64, skuIDs []int64) error
	GetCart(ctx context.Context, userID int64) (domain.Cart, error)
	EmptyCart(ctx context.Context, userID int64) error
	ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error
	IncrementQty(ctx context.Context, userID, skuID int64) (newQuantity int64, err error)
	DecrementQty(ctx context.Context, userID, skuID int64) (newQuantity int64, err error)
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

func (r *cachedCartRepository) productMapKey(userID int64) string {
	return fmt.Sprintf("cart:%d:products", userID)
}

func (r *cachedCartRepository) skuField(skuID int64) string {
	return strconv.FormatInt(skuID, 10)
}

func (r *cachedCartRepository) AddItems(ctx context.Context, userID int64, items []domain.CartItem) error {
	if len(items) == 0 {
		return nil
	}

	key := r.cartKey(userID)
	productKey := r.productMapKey(userID)
	quantityBySKU := make(map[string]int64, len(items))
	productBySKU := make(map[string]int64, len(items))

	for _, item := range items {
		if item.SKUID == 0 {
			return fmt.Errorf("sku_id is required")
		}
		if item.ProductID == 0 {
			return fmt.Errorf("product_id is required")
		}
		field := r.skuField(item.SKUID)
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		quantityBySKU[field] += quantity
		productBySKU[field] = item.ProductID
	}

	if _, err := r.cache.HIncrByBatch(ctx, key, quantityBySKU); err != nil {
		return err
	}
	if err := r.cache.HSetBatch(ctx, productKey, productBySKU); err != nil {
		return err
	}

	go r.asyncUpsertIncrementBatch(userID, items)
	return nil
}

func (r *cachedCartRepository) DeleteItems(ctx context.Context, userID int64, skuIDs []int64) error {
	if len(skuIDs) == 0 {
		return nil
	}

	fields := make([]string, len(skuIDs))
	for i, skuID := range skuIDs {
		fields[i] = r.skuField(skuID)
	}
	if err := r.cache.HDel(ctx, r.cartKey(userID), fields...); err != nil {
		return err
	}
	if err := r.cache.HDel(ctx, r.productMapKey(userID), fields...); err != nil {
		return err
	}

	go r.asyncDeleteBySKUIDs(userID, skuIDs)
	return nil
}

func (r *cachedCartRepository) GetCart(ctx context.Context, userID int64) (domain.Cart, error) {
	result, err := r.cache.HGetAll(ctx, r.cartKey(userID))
	if err == nil && len(result) > 0 {
		productResult, err := r.cache.HGetAll(ctx, r.productMapKey(userID))
		if err != nil {
			return domain.Cart{}, err
		}
		cart := r.mapToCart(userID, result, productResult)
		if len(cart.Items) > 0 {
			return cart, nil
		}
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
			SKUID:     daoItem.SKUID,
			Quantity:  daoItem.Quantity,
		})
	}

	return domain.Cart{
		UserID: userID,
		Items:  items,
	}, nil
}

func (r *cachedCartRepository) EmptyCart(ctx context.Context, userID int64) error {
	if err := r.cache.Del(ctx, r.cartKey(userID), r.productMapKey(userID)); err != nil {
		return err
	}

	go r.asyncEmptyCart(userID)
	return nil
}

func (r *cachedCartRepository) ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error {
	if item.SKUID == 0 {
		return fmt.Errorf("sku_id is required")
	}
	if item.ProductID == 0 {
		return fmt.Errorf("product_id is required")
	}

	field := r.skuField(item.SKUID)
	if err := r.cache.HSet(ctx, r.cartKey(userID), field, item.Quantity); err != nil {
		return err
	}
	if err := r.cache.HSet(ctx, r.productMapKey(userID), field, item.ProductID); err != nil {
		return err
	}

	go r.asyncWriteToMySQL(userID, item)
	return nil
}

func (r *cachedCartRepository) IncrementQty(ctx context.Context, userID, skuID int64) (int64, error) {
	if skuID == 0 {
		return 0, fmt.Errorf("sku_id is required")
	}

	newQty, err := r.cache.HIncrBy(ctx, r.cartKey(userID), r.skuField(skuID), 1)
	if err != nil {
		return 0, err
	}

	go r.asyncIncrementToMySQL(userID, skuID)
	return newQty, nil
}

func (r *cachedCartRepository) DecrementQty(ctx context.Context, userID, skuID int64) (int64, error) {
	if skuID == 0 {
		return 0, fmt.Errorf("sku_id is required")
	}

	newQty, err := r.cache.DecrementIfGreaterThanOne(ctx, r.cartKey(userID), r.skuField(skuID))
	if err != nil {
		return 0, err
	}

	go r.asyncDecrementToMySQL(userID, skuID)
	return newQty, nil
}

func (r *cachedCartRepository) mapToCart(userID int64, quantities map[string]string, products map[string]string) domain.Cart {
	items := make([]domain.CartItem, 0, len(quantities))
	for skuIDStr, quantityStr := range quantities {
		skuID, err := strconv.ParseInt(skuIDStr, 10, 64)
		if err != nil {
			continue
		}
		quantity, err := strconv.ParseInt(quantityStr, 10, 64)
		if err != nil || quantity <= 0 {
			continue
		}
		productID, err := strconv.ParseInt(products[skuIDStr], 10, 64)
		if err != nil || productID == 0 {
			continue
		}
		items = append(items, domain.CartItem{
			ProductID: productID,
			SKUID:     skuID,
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
		SKUID:     item.SKUID,
		Quantity:  item.Quantity,
	}

	if err := r.dao.Upsert(ctx, daoItem); err != nil {
		r.logger.Error("write cart item to MySQL failed", logger.Int64("user_id", userID), logger.Int64("sku_id", item.SKUID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncUpsertIncrementBatch(userID int64, items []domain.CartItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	daoItems := make([]dao.CartItem, 0, len(items))
	for _, item := range items {
		daoItems = append(daoItems, dao.CartItem{
			UserID:    userID,
			ProductID: item.ProductID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}
	if err := r.dao.UpsertIncrementBatch(ctx, userID, daoItems); err != nil {
		r.logger.Error("increment cart items in MySQL failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncDeleteBySKUIDs(userID int64, skuIDs []int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.dao.DeleteBySKUIDs(ctx, userID, skuIDs); err != nil {
		r.logger.Error("delete cart items from MySQL failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncIncrementToMySQL(userID, skuID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.dao.IncrementQuantity(ctx, userID, skuID); err != nil {
		r.logger.Error("increment cart item in MySQL failed", logger.Int64("user_id", userID), logger.Int64("sku_id", skuID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncDecrementToMySQL(userID, skuID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.dao.DecrementQuantity(ctx, userID, skuID); err != nil {
		r.logger.Error("decrement cart item in MySQL failed", logger.Int64("user_id", userID), logger.Int64("sku_id", skuID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncEmptyCart(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.dao.DeleteByUserID(ctx, userID); err != nil {
		r.logger.Error("empty cart in MySQL failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (r *cachedCartRepository) asyncWriteBackToRedis(userID int64, daoItems []dao.CartItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	quantities := make(map[string]int64, len(daoItems))
	products := make(map[string]int64, len(daoItems))
	for _, item := range daoItems {
		field := r.skuField(item.SKUID)
		quantities[field] = item.Quantity
		products[field] = item.ProductID
	}

	if err := r.cache.HSetBatch(ctx, r.cartKey(userID), quantities); err != nil {
		r.logger.Error("write cart quantities back to Redis failed", logger.Int64("user_id", userID), logger.Error(err))
		return
	}
	if err := r.cache.HSetBatch(ctx, r.productMapKey(userID), products); err != nil {
		r.logger.Error("write cart products back to Redis failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}
