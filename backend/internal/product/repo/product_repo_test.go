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

// =============================================================================
// ProductRepo 接口测试（可复用于 CacheAside 和 Canal 两种实现）
// =============================================================================

// 创建测试用的 Repo（可切换实现）
func newTestRepo(t *testing.T, dao dao.ProductDao, cache cache.ProductCache) ProductRepo {
	// 切换实现：CacheAside 或 Canal
	return NewCacheAsideProductRepo(dao, cache, logger.NewNopLogger())
	// return NewCachedProductRepo(dao, cache, logger.NewNopLogger())
}

// =============================================================================
// GetProduct 测试
// =============================================================================

func TestProductRepo_GetProduct(t *testing.T) {
	testCases := []struct {
		name      string
		id        int64
		mock      func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantErr   bool
		wantPrice int64
	}{
		{
			name: "缓存全部命中",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 基本信息缓存命中
				basicProduct := domain.Product{
					ID:   1,
					Name: "测试商品",
				}
				basicData, _ := json.Marshal(basicProduct)
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)

				// 价格/库存缓存命中
				ps := PriceStock{Price: 9900, Stock: 100}
				psData, _ := json.Marshal(ps)
				c.EXPECT().Get(gomock.Any(), PriceStockKey(1)).Return(psData, nil)

				// 不应该查数据库
				return d, c
			},
			wantErr:   false,
			wantPrice: 9900,
		},
		{
			name: "基本信息命中_价格未命中_精准查询",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 基本信息缓存命中
				basicProduct := domain.Product{
					ID:   1,
					Name: "测试商品",
				}
				basicData, _ := json.Marshal(basicProduct)
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)

				// 价格/库存缓存未命中
				c.EXPECT().Get(gomock.Any(), PriceStockKey(1)).Return(nil, errors.New("not found"))

				// 只查价格/库存（精准查询优化）
				d.EXPECT().FindPriceStock(gomock.Any(), int64(1)).Return(int64(8800), int64(50), nil)

				// 回填价格/库存缓存
				c.EXPECT().SetWithTTL(gomock.Any(), PriceStockKey(1), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantErr:   false,
			wantPrice: 8800,
		},
		{
			name: "缓存全部未命中_查库回填",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缓存都未命中
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("not found"))
				c.EXPECT().Get(gomock.Any(), PriceStockKey(1)).Return(nil, errors.New("not found"))

				// 查库
				d.EXPECT().FindByID(gomock.Any(), int64(1)).Return(dao.Product{
					ID:    1,
					Name:  "测试商品",
					Price: 7700,
					Stock: 30,
				}, nil)

				// 回填两个缓存
				c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(1), gomock.Any(), gomock.Any()).Return(nil)
				c.EXPECT().SetWithTTL(gomock.Any(), PriceStockKey(1), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantErr:   false,
			wantPrice: 7700,
		},
		{
			name: "数据库查询失败",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("not found"))
				c.EXPECT().Get(gomock.Any(), PriceStockKey(1)).Return(nil, errors.New("not found"))

				d.EXPECT().FindByID(gomock.Any(), int64(1)).Return(dao.Product{}, errors.New("db error"))

				return d, c
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := newTestRepo(t, d, c)

			product, err := repo.GetProduct(context.Background(), tc.id)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantPrice, product.Price)
			}
		})
	}
}

// =============================================================================
// CreateProduct 测试
// =============================================================================

func TestProductRepo_CreateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		product domain.Product
		mock    func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantID  int64
		wantErr bool
	}{
		{
			name: "创建成功_回填缓存",
			product: domain.Product{
				Name:  "新商品",
				Price: 9900,
				Stock: 100,
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(int64(123), nil)

				// 回填缓存
				c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(123), gomock.Any(), gomock.Any()).Return(nil)
				c.EXPECT().SetWithTTL(gomock.Any(), PriceStockKey(123), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantID:  123,
			wantErr: false,
		},
		{
			name: "数据库插入失败",
			product: domain.Product{
				Name: "新商品",
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("db error"))

				return d, c
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := newTestRepo(t, d, c)

			id, err := repo.CreateProduct(context.Background(), tc.product)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, id)
			}
		})
	}
}

// =============================================================================
// UpdateProduct 测试（延迟双删）
// =============================================================================

func TestProductRepo_UpdateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		product domain.Product
		mock    func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantErr bool
	}{
		{
			name: "更新成功_延迟双删",
			product: domain.Product{
				ID:    1,
				Name:  "更新后的商品",
				Price: 8800,
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

				// 第一次删除（立即）
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil)
				c.EXPECT().Delete(gomock.Any(), PriceStockKey(1)).Return(nil)

				// 第二次删除（延迟，异步）- 使用 AnyTimes 因为是异步的
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()
				c.EXPECT().Delete(gomock.Any(), PriceStockKey(1)).Return(nil).AnyTimes()

				return d, c
			},
			wantErr: false,
		},
		{
			name: "更新失败_不删缓存",
			product: domain.Product{
				ID:   1,
				Name: "更新后的商品",
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
				// 不应该删缓存

				return d, c
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := newTestRepo(t, d, c)

			_, err := repo.UpdateProduct(context.Background(), tc.product)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				// 等待延迟双删完成
				time.Sleep(1500 * time.Millisecond)
			}
		})
	}
}

// =============================================================================
// DeleteProduct 测试
// =============================================================================

func TestProductRepo_DeleteProduct(t *testing.T) {
	testCases := []struct {
		name    string
		id      int64
		userID  int64
		mock    func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantErr bool
	}{
		{
			name:   "删除成功",
			id:     1,
			userID: 100,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Delete(gomock.Any(), int64(1), int64(100)).Return(nil)

				// 删缓存
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil)
				c.EXPECT().Delete(gomock.Any(), PriceStockKey(1)).Return(nil)

				// 延迟双删
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()
				c.EXPECT().Delete(gomock.Any(), PriceStockKey(1)).Return(nil).AnyTimes()

				return d, c
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := newTestRepo(t, d, c)

			err := repo.DeleteProduct(context.Background(), tc.id, tc.userID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				time.Sleep(1500 * time.Millisecond)
			}
		})
	}
}

// =============================================================================
// ListProducts 测试（singleflight + 预热）
// =============================================================================

func TestProductRepo_ListProducts(t *testing.T) {
	testCases := []struct {
		name      string
		page      int64
		pageSize  int64
		category  string
		mock      func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantCount int
		wantErr   bool
	}{
		{
			name:     "热点数据_缓存命中",
			page:     1,
			pageSize: 10,
			category: "电子产品",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缓存命中
				products := []domain.Product{
					{ID: 1, Name: "商品1", Price: 100},
					{ID: 2, Name: "商品2", Price: 200},
				}
				data, _ := json.Marshal(products)
				c.EXPECT().Get(gomock.Any(), ListKey("电子产品", 1)).Return(data, nil)

				// 预热相关调用（异步，使用 AnyTimes）
				c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found")).AnyTimes()
				c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
				c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				return d, c
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "热点数据_缓存未命中_查库回填",
			page:     1,
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缓存未命中
				c.EXPECT().Get(gomock.Any(), ListKey("", 1)).Return(nil, errors.New("not found"))

				// 查库
				d.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]dao.Product{
					{ID: 1, Name: "商品1", Price: 100},
					{ID: 2, Name: "商品2", Price: 200},
					{ID: 3, Name: "商品3", Price: 300},
				}, nil)

				// 回填缓存
				c.EXPECT().SetWithTTL(gomock.Any(), ListKey("", 1), gomock.Any(), gomock.Any()).Return(nil)

				// 预热相关调用
				c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found")).AnyTimes()
				c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
				c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				return d, c
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:     "非热点数据_直接查库",
			page:     5, // 第5页，非热点
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 非热点数据，直接查库，不走缓存
				d.EXPECT().ListProducts(gomock.Any(), int64(5), int64(10), "").Return([]dao.Product{
					{ID: 41, Name: "商品41", Price: 100},
				}, nil)

				return d, c
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := newTestRepo(t, d, c)

			products, err := repo.ListProducts(context.Background(), tc.page, tc.pageSize, tc.category)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, products, tc.wantCount)
			}

			// 等待异步预热完成
			time.Sleep(100 * time.Millisecond)
		})
	}
}
