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
			name:     "成功获取商品列表",
			page:     1,
			pageSize: 10,
			category: "",
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]domain.Product{
					{ID: 1, Name: "商品1", Price: 100},
					{ID: 2, Name: "商品2", Price: 200},
				}, nil)
				return repo
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "按分类查询",
			page:     1,
			pageSize: 10,
			category: "电子产品",
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				repo := repomocks.NewMockProductRepo(ctrl)
				repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "电子产品").Return([]domain.Product{
					{ID: 1, Name: "手机", Price: 599900, Categories: []string{"电子产品"}},
				}, nil)
				return repo
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:     "Repo返回错误",
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
			name: "成功获取商品详情",
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
			name: "商品不存在",
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
			name: "成功创建商品",
			product: domain.Product{
				Name:       "新商品",
				Price:      9900,
				InStock:    true,
				Categories: []string{"服装"},
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
			name: "商品名包含敏感词",
			product: domain.Product{
				Name:  "敏感词商品",
				Price: 9900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				// 不会调用 repo，因为敏感词校验在前
				return repomocks.NewMockProductRepo(ctrl)
			},
			wantID:  0,
			wantErr: true,
		},
		{
			name: "描述包含敏感词",
			product: domain.Product{
				Name:        "正常商品",
				Description: "这是敏感词描述",
				Price:       9900,
			},
			mock: func(ctrl *gomock.Controller) *repomocks.MockProductRepo {
				return repomocks.NewMockProductRepo(ctrl)
			},
			wantID:  0,
			wantErr: true,
		},
		{
			name: "Repo返回错误",
			product: domain.Product{
				Name:  "正常商品",
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
			name: "成功更新商品",
			product: domain.Product{
				ID:    1,
				Name:  "更新后的商品名",
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
			name: "更新时商品名包含敏感词",
			product: domain.Product{
				ID:   1,
				Name: "敏感词名称",
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
			name:   "成功删除商品",
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
			name:   "删除失败-无权限",
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
