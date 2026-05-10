package repo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	cachemocks "github.com/XDWow/DouyinMall/backend/internal/product/repo/cache/mocks"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	daomocks "github.com/XDWow/DouyinMall/backend/internal/product/repo/dao/mocks"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func newTestRepo(dao dao.ProductDao, cache cache.ProductCache) ProductRepo {
	return NewCachedProductRepo(dao, cache, logger.NewNopLogger())
}

func TestProductRepo_GetProduct_FullCacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	query := domain.ProductQuery{ID: 1, SKUID: 101}

	basicData, _ := json.Marshal(domain.Product{ID: 1, Name: "Phone"})
	priceData, _ := json.Marshal(PriceInStock{SKUID: 101, Price: 9900, Currency: "CNY", InStock: true})
	c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)
	c.EXPECT().Get(gomock.Any(), PriceInStockKey(1, 101)).Return(priceData, nil)

	product, err := newTestRepo(d, c).GetProduct(context.Background(), query)

	require.NoError(t, err)
	assert.Equal(t, int64(101), product.SKUID)
	assert.Equal(t, int64(9900), product.Price)
	assert.True(t, product.InStock)
}

func TestProductRepo_GetProduct_BasicCacheHitPriceMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	query := domain.ProductQuery{ID: 1, SKUID: 101}

	basicData, _ := json.Marshal(domain.Product{ID: 1, Name: "Phone"})
	c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)
	c.EXPECT().Get(gomock.Any(), PriceInStockKey(1, 101)).Return(nil, errors.New("cache miss"))
	d.EXPECT().FindPriceInStock(gomock.Any(), int64(1), int64(101)).Return(int64(8800), "CNY", true, nil)
	c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(1, 101), gomock.Any(), gomock.Any()).Return(nil)

	product, err := newTestRepo(d, c).GetProduct(context.Background(), query)

	require.NoError(t, err)
	assert.Equal(t, int64(8800), product.Price)
	assert.Equal(t, int64(101), product.SKUID)
}

func TestProductRepo_GetProduct_FullMissLoadsDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	query := domain.ProductQuery{ID: 1, SKUID: 101}

	c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("cache miss"))
	c.EXPECT().Get(gomock.Any(), PriceInStockKey(1, 101)).Return(nil, errors.New("cache miss"))
	d.EXPECT().FindByID(gomock.Any(), int64(1)).Return(dao.Product{
		ID:       1,
		Name:     "Phone",
		Price:    7700,
		Currency: "CNY",
		InStock:  true,
	}, nil)
	d.EXPECT().FindPriceInStock(gomock.Any(), int64(1), int64(101)).Return(int64(6600), "CNY", true, nil)
	c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(1), gomock.Any(), gomock.Any()).Return(nil)
	c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(1, 101), gomock.Any(), gomock.Any()).Return(nil)

	product, err := newTestRepo(d, c).GetProduct(context.Background(), query)

	require.NoError(t, err)
	assert.Equal(t, int64(6600), product.Price)
	assert.Equal(t, int64(101), product.SKUID)
}

func TestProductRepo_GetProduct_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)

	c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("cache miss"))
	c.EXPECT().Get(gomock.Any(), PriceInStockKey(1, 101)).Return(nil, errors.New("cache miss"))
	d.EXPECT().FindByID(gomock.Any(), int64(1)).Return(dao.Product{}, errors.New("db error"))

	_, err := newTestRepo(d, c).GetProduct(context.Background(), domain.ProductQuery{ID: 1, SKUID: 101})
	assert.Error(t, err)
}

func TestProductRepo_CreateProduct_UpsertsSKUAndCaches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	product := domain.Product{
		SKUID:      101,
		Name:       "Phone",
		Price:      9900,
		Currency:   "CNY",
		InStock:    true,
		MerchantID: 1001,
	}

	d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(int64(123), nil)
	d.EXPECT().UpsertSKU(gomock.Any(), gomock.Any()).Return(nil)
	c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(123), gomock.Any(), gomock.Any()).Return(nil)
	c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(123, 101), gomock.Any(), gomock.Any()).Return(nil)

	id, err := newTestRepo(d, c).CreateProduct(context.Background(), product)

	require.NoError(t, err)
	assert.Equal(t, int64(123), id)
}

func TestProductRepo_UpdateProduct_InvalidatesSKUCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	product := domain.Product{
		ID:         1,
		SKUID:      101,
		Name:       "Phone Pro",
		Price:      12900,
		Currency:   "CNY",
		InStock:    true,
		MerchantID: 1001,
	}

	d.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.EXPECT().UpsertSKU(gomock.Any(), gomock.Any()).Return(nil)
	c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()
	c.EXPECT().Delete(gomock.Any(), PriceInStockKey(1, 101)).Return(nil).AnyTimes()

	id, err := newTestRepo(d, c).UpdateProduct(context.Background(), product)

	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	time.Sleep(1100 * time.Millisecond)
}

func TestProductRepo_DeleteProduct_RemovesSKUAndCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)

	d.EXPECT().Delete(gomock.Any(), int64(1), int64(1001)).Return(nil)
	d.EXPECT().DeleteSKUsByProductID(gomock.Any(), int64(1)).Return(nil)
	c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()

	err := newTestRepo(d, c).DeleteProduct(context.Background(), 1, 1001)

	require.NoError(t, err)
	time.Sleep(1100 * time.Millisecond)
}

func TestProductRepo_ListProducts_UsesPageSizeInCacheKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)
	products := []domain.Product{{ID: 1, Name: "Phone", Price: 100}}
	data, _ := json.Marshal(products)

	c.EXPECT().Get(gomock.Any(), ListKey("electronics", 1, 10)).Return(data, nil)
	c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("cache miss")).AnyTimes()
	c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
	c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	got, err := newTestRepo(d, c).ListProducts(context.Background(), 1, 10, "electronics")

	require.NoError(t, err)
	assert.Len(t, got, 1)
	time.Sleep(100 * time.Millisecond)
}

func TestProductRepo_ListProducts_MissLoadsDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := daomocks.NewMockProductDao(ctrl)
	c := cachemocks.NewMockProductCache(ctrl)

	c.EXPECT().Get(gomock.Any(), ListKey("", 1, 10)).Return(nil, errors.New("cache miss"))
	d.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]dao.Product{
		{ID: 1, Name: "Phone", Price: 100, Currency: "CNY", InStock: true},
	}, nil)
	c.EXPECT().SetWithTTL(gomock.Any(), ListKey("", 1, 10), gomock.Any(), gomock.Any()).Return(nil)
	c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("cache miss")).AnyTimes()
	c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
	c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	got, err := newTestRepo(d, c).ListProducts(context.Background(), 1, 10, "")

	require.NoError(t, err)
	assert.Len(t, got, 1)
	time.Sleep(100 * time.Millisecond)
}
