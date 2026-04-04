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
// ProductRepo 鎺ュ彛娴嬭瘯锛堝彲澶嶇敤浜?CacheAside 鍜?Canal 涓ょ瀹炵幇锛?
// =============================================================================

// 鍒涘缓娴嬭瘯鐢ㄧ殑 Repo锛堝彲鍒囨崲瀹炵幇锛?
func newTestRepo(t *testing.T, dao dao.ProductDao, cache cache.ProductCache) ProductRepo {
	// 鍒囨崲瀹炵幇锛欳acheAside 鎴?Canal
	return NewCachedProductRepo(dao, cache, logger.NewNopLogger())
	// return NewCachedProductRepo(dao, cache, logger.NewNopLogger())
}

// =============================================================================
// GetProduct 娴嬭瘯
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
			name: "缂撳瓨鍏ㄩ儴鍛戒腑",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 鍩烘湰淇℃伅缂撳瓨鍛戒腑
				basicProduct := domain.Product{
					ID:   1,
					Name: "娴嬭瘯鍟嗗搧",
				}
				basicData, _ := json.Marshal(basicProduct)
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)

				// 浠锋牸/搴撳瓨鐘舵€佺紦瀛樺懡涓?
				ps := PriceInStock{Price: 9900, InStock: true}
				psData, _ := json.Marshal(ps)
				c.EXPECT().Get(gomock.Any(), PriceInStockKey(1)).Return(psData, nil)

				// 涓嶅簲璇ユ煡鏁版嵁搴?
				return d, c
			},
			wantErr:   false,
			wantPrice: 9900,
		},
		{
			name: "鍩烘湰淇℃伅鍛戒腑_浠锋牸鏈懡涓璤绮惧噯鏌ヨ",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 鍩烘湰淇℃伅缂撳瓨鍛戒腑
				basicProduct := domain.Product{
					ID:   1,
					Name: "娴嬭瘯鍟嗗搧",
				}
				basicData, _ := json.Marshal(basicProduct)
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(basicData, nil)

				// 浠锋牸/搴撳瓨鐘舵€佺紦瀛樻湭鍛戒腑
				c.EXPECT().Get(gomock.Any(), PriceInStockKey(1)).Return(nil, errors.New("not found"))

				// 鍙煡浠锋牸/搴撳瓨鐘舵€侊紙绮惧噯鏌ヨ浼樺寲锛?
				d.EXPECT().FindPriceInStock(gomock.Any(), int64(1)).Return(int64(8800), true, nil)

				// 鍥炲～浠锋牸/搴撳瓨鐘舵€佺紦瀛?
				c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(1), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantErr:   false,
			wantPrice: 8800,
		},
		{
			name: "缂撳瓨鍏ㄩ儴鏈懡涓璤鏌ュ簱鍥炲～",
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缂撳瓨閮芥湭鍛戒腑
				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("not found"))
				c.EXPECT().Get(gomock.Any(), PriceInStockKey(1)).Return(nil, errors.New("not found"))

				// 鏌ュ簱
				d.EXPECT().FindByID(gomock.Any(), int64(1)).Return(dao.Product{
					ID:      1,
					Name:    "娴嬭瘯鍟嗗搧",
					Price:   7700,
					InStock: true,
				}, nil)

				// 鍥炲～涓や釜缂撳瓨
				c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(1), gomock.Any(), gomock.Any()).Return(nil)
				c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(1), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantErr:   false,
			wantPrice: 7700,
		},
		{
			name: "鏁版嵁搴撴煡璇㈠け璐?,
			id:   1,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				c.EXPECT().Get(gomock.Any(), DetailBasicKey(1)).Return(nil, errors.New("not found"))
				c.EXPECT().Get(gomock.Any(), PriceInStockKey(1)).Return(nil, errors.New("not found"))

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
// CreateProduct 娴嬭瘯
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
			name: "鍒涘缓鎴愬姛_鍥炲～缂撳瓨",
			product: domain.Product{
				Name:    "鏂板晢鍝?,
				Price:   9900,
				InStock: true,
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(int64(123), nil)

				// 鍥炲～缂撳瓨
				c.EXPECT().SetWithTTL(gomock.Any(), DetailBasicKey(123), gomock.Any(), gomock.Any()).Return(nil)
				c.EXPECT().SetWithTTL(gomock.Any(), PriceInStockKey(123), gomock.Any(), gomock.Any()).Return(nil)

				return d, c
			},
			wantID:  123,
			wantErr: false,
		},
		{
			name: "鏁版嵁搴撴彃鍏ュけ璐?,
			product: domain.Product{
				Name: "鏂板晢鍝?,
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
// UpdateProduct 娴嬭瘯锛堝欢杩熷弻鍒狅級
// =============================================================================

func TestProductRepo_UpdateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		product domain.Product
		mock    func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache)
		wantErr bool
	}{
		{
			name: "鏇存柊鎴愬姛_寤惰繜鍙屽垹",
			product: domain.Product{
				ID:    1,
				Name:  "鏇存柊鍚庣殑鍟嗗搧",
				Price: 8800,
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

				// 绗竴娆″垹闄わ紙绔嬪嵆锛?
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil)
				c.EXPECT().Delete(gomock.Any(), PriceInStockKey(1)).Return(nil)

				// 绗簩娆″垹闄わ紙寤惰繜锛屽紓姝ワ級- 浣跨敤 AnyTimes 鍥犱负鏄紓姝ョ殑
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()
				c.EXPECT().Delete(gomock.Any(), PriceInStockKey(1)).Return(nil).AnyTimes()

				return d, c
			},
			wantErr: false,
		},
		{
			name: "鏇存柊澶辫触_涓嶅垹缂撳瓨",
			product: domain.Product{
				ID:   1,
				Name: "鏇存柊鍚庣殑鍟嗗搧",
			},
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
				// 涓嶅簲璇ュ垹缂撳瓨

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
				// 绛夊緟寤惰繜鍙屽垹瀹屾垚
				time.Sleep(1500 * time.Millisecond)
			}
		})
	}
}

// =============================================================================
// DeleteProduct 娴嬭瘯
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
			name:   "鍒犻櫎鎴愬姛",
			id:     1,
			userID: 100,
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				d.EXPECT().Delete(gomock.Any(), int64(1), int64(100)).Return(nil)

				// 鍒犵紦瀛?
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil)
				c.EXPECT().Delete(gomock.Any(), PriceInStockKey(1)).Return(nil)

				// 寤惰繜鍙屽垹
				c.EXPECT().Delete(gomock.Any(), DetailBasicKey(1)).Return(nil).AnyTimes()
				c.EXPECT().Delete(gomock.Any(), PriceInStockKey(1)).Return(nil).AnyTimes()

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
// ListProducts 娴嬭瘯锛坰ingleflight + 棰勭儹锛?
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
			name:     "鐑偣鏁版嵁_缂撳瓨鍛戒腑",
			page:     1,
			pageSize: 10,
			category: "鐢靛瓙浜у搧",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缂撳瓨鍛戒腑
				products := []domain.Product{
					{ID: 1, Name: "鍟嗗搧1", Price: 100},
					{ID: 2, Name: "鍟嗗搧2", Price: 200},
				}
				data, _ := json.Marshal(products)
				c.EXPECT().Get(gomock.Any(), ListKey("鐢靛瓙浜у搧", 1)).Return(data, nil)

				// 棰勭儹鐩稿叧璋冪敤锛堝紓姝ワ紝浣跨敤 AnyTimes锛?
				c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found")).AnyTimes()
				c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
				c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				return d, c
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "鐑偣鏁版嵁_缂撳瓨鏈懡涓璤鏌ュ簱鍥炲～",
			page:     1,
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 缂撳瓨鏈懡涓?
				c.EXPECT().Get(gomock.Any(), ListKey("", 1)).Return(nil, errors.New("not found"))

				// 鏌ュ簱
				d.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]dao.Product{
					{ID: 1, Name: "鍟嗗搧1", Price: 100},
					{ID: 2, Name: "鍟嗗搧2", Price: 200},
					{ID: 3, Name: "鍟嗗搧3", Price: 300},
				}, nil)

				// 鍥炲～缂撳瓨
				c.EXPECT().SetWithTTL(gomock.Any(), ListKey("", 1), gomock.Any(), gomock.Any()).Return(nil)

				// 棰勭儹鐩稿叧璋冪敤
				c.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found")).AnyTimes()
				c.EXPECT().GetTTL(gomock.Any(), gomock.Any()).Return(time.Hour, nil).AnyTimes()
				c.EXPECT().BatchSetWithTTL(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				return d, c
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:     "闈炵儹鐐规暟鎹甠鐩存帴鏌ュ簱",
			page:     5, // 绗?椤碉紝闈炵儹鐐?
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) (dao.ProductDao, cache.ProductCache) {
				d := daomocks.NewMockProductDao(ctrl)
				c := cachemocks.NewMockProductCache(ctrl)

				// 闈炵儹鐐规暟鎹紝鐩存帴鏌ュ簱锛屼笉璧扮紦瀛?
				d.EXPECT().ListProducts(gomock.Any(), int64(5), int64(10), "").Return([]dao.Product{
					{ID: 41, Name: "鍟嗗搧41", Price: 100},
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

			// 绛夊緟寮傛棰勭儹瀹屾垚
			time.Sleep(100 * time.Millisecond)
		})
	}
}


