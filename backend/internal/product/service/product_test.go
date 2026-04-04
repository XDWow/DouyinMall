package service

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	repomocks "github.com/XDWow/DouyinMall/backend/internal/product/repo/mocks"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProductService_ListProducts(t *testing.T) {
	testCases := []struct {
		name      string
		page      int64
		pageSize  int64
		category  string
		mock      func(ctrl *gomock.Controller) *repomocks.MockProductRepo
		wantCount int
		wantErr   bool
	}{
		{
			name:     "鎴愬姛鑾峰彇鍟嗗搧鍒楄〃",
			page:     1,
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]domain.Product{
					{ID: 1, Name: "鍟嗗搧1", Price: 100},
					{ID: 2, Name: "鍟嗗搧2", Price: 200},
				}, nil)
				return repo
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "鎸夊垎绫绘煡璇?,
			page:     1,
			pageSize: 10,
			category: "鐢靛瓙浜у搧",
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "鐢靛瓙浜у搧").Return([]domain.Product{
					{ID: 1, Name: "鎵嬫満", Price: 599900, Categories: []string{"鐢靛瓙浜у搧"}},
				}, nil)
				return repo
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:     "Repo杩斿洖閿欒",
			page:     1,
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return(nil, errors.New("db error"))
				return repo
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewProductService(repo, logger.NewNopLogger())

			products, err := svc.ListProducts(context.Background(), tc.page, tc.pageSize, tc.category)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, products, tc.wantCount)
			}
		})
	}
}

func TestProductService_GetProduct(t *testing.T) {
	testCases := []struct {
		name    string
		id      int64
		mock    func(ctrl *gomock.Controller) *repomocks.MockProductRepo
		wantID  int64
		wantErr bool
	}{
		{
			name: "鎴愬姛鑾峰彇鍟嗗搧璇︽儏",
			id:   1,
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().GetProduct(gomock.Any(), int64(1)).Return(domain.Product{
					ID:      1,
					Name:    "iPhone 15",
					Price:   599900,
					InStock: true,
				}, nil)
				return repo
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "鍟嗗搧涓嶅瓨鍦?,
			id:   999,
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().GetProduct(gomock.Any(), int64(999)).Return(domain.Product{}, errors.New("not found"))
				return repo
			},
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewProductService(repo, logger.NewNopLogger())

			product, err := svc.GetProduct(context.Background(), tc.id)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, product.ID)
			}
		})
	}
}

func TestProductService_CreateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		product domain.Product
		mock    func(ctrl *gomock.Controller) *repomocks.MockProductRepo
		wantID  int64
		wantErr bool
	}{
		{
			name: "鎴愬姛鍒涘缓鍟嗗搧",
			product: domain.Product{
				Name:       "鏂板晢鍝?,
				Price:      9900,
				InStock:    true,
				Categories: []string{"鏈嶈"},
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().CreateProduct(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				return repo
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "鍟嗗搧鍚嶅寘鍚晱鎰熻瘝",
			product: domain.Product{
				Name:  "鏁忔劅璇嶅晢鍝?,
				Price: 9900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				// 涓嶄細璋冪敤 repo锛屽洜涓烘晱鎰熻瘝鏍￠獙鍦ㄥ墠
				return repomocks.NewMockProductRepo(ctrl)
			},
			wantID:  0,
			wantErr: true,
		},
		{
			name: "鎻忚堪鍖呭惈鏁忔劅璇?,
			product: domain.Product{
				Name:        "姝ｅ父鍟嗗搧",
				Description: "杩欐槸鏁忔劅璇嶆弿杩?,
				Price:       9900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				return repomocks.NewMockProductRepo(ctrl)
			},
			wantID:  0,
			wantErr: true,
		},
		{
			name: "Repo杩斿洖閿欒",
			product: domain.Product{
				Name:  "姝ｅ父鍟嗗搧",
				Price: 9900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().CreateProduct(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("db error"))
				return repo
			},
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewProductService(repo, logger.NewNopLogger())

			id, err := svc.CreateProduct(context.Background(), tc.product)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, id)
			}
		})
	}
}

func TestProductService_UpdateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		product domain.Product
		mock    func(ctrl *gomock.Controller) *repomocks.MockProductRepo
		wantID  int64
		wantErr bool
	}{
		{
			name: "鎴愬姛鏇存柊鍟嗗搧",
			product: domain.Product{
				ID:    1,
				Name:  "鏇存柊鍚庣殑鍟嗗搧鍚?,
				Price: 19900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().UpdateProduct(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				return repo
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "鏇存柊鏃跺晢鍝佸悕鍖呭惈鏁忔劅璇?,
			product: domain.Product{
				ID:   1,
				Name: "鏁忔劅璇嶅悕绉?,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				return repomocks.NewMockProductRepo(ctrl)
			},
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewProductService(repo, logger.NewNopLogger())

			id, err := svc.UpdateProduct(context.Background(), tc.product)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, id)
			}
		})
	}
}

func TestProductService_DeleteProduct(t *testing.T) {
	testCases := []struct {
		name    string
		id      int64
		userID  int64
		mock    func(ctrl *gomock.Controller) *repomocks.MockProductRepo
		wantErr bool
	}{
		{
			name:   "鎴愬姛鍒犻櫎鍟嗗搧",
			id:     1,
			userID: 100,
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(100)).Return(nil)
				return repo
			},
			wantErr: false,
		},
		{
			name:   "鍒犻櫎澶辫触-鏃犳潈闄?,
			id:     1,
			userID: 999,
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(999)).Return(errors.New("no permission"))
				return repo
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewProductService(repo, logger.NewNopLogger())

			err := svc.DeleteProduct(context.Background(), tc.id, tc.userID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}


