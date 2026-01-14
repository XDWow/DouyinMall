package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	svcmocks "github.com/XDWow/DouyinMall/backend/internal/product/service/mocks"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProductHandler_ListProducts(t *testing.T) {
	testCases := []struct {
		name      string
		req       *v1.ListProductsReq
		mock      func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantCount int
		wantErr   bool
	}{
		{
			name: "成功获取商品列表",
			req: &v1.ListProductsReq{
				Page:     1,
				PageSize: 10,
				Category: "",
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]domain.Product{
					{ID: 1, Name: "商品1", Price: 100},
					{ID: 2, Name: "商品2", Price: 200},
				}, nil)
				return svc
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "Service返回错误",
			req: &v1.ListProductsReq{
				Page:     1,
				PageSize: 10,
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return(nil, errors.New("service error"))
				return svc
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewProductHandler(svc)

			resp, err := h.ListProducts(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.Len(t, resp.Products, tc.wantCount)
			}
		})
	}
}

func TestProductHandler_GetProduct(t *testing.T) {
	testCases := []struct {
		name       string
		req        *v1.GetProductReq
		mock       func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantID     int64
		wantName   string
		wantErr    bool
	}{
		{
			name: "成功获取商品详情",
			req:  &v1.GetProductReq{Id: 1},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().GetProduct(gomock.Any(), int64(1)).Return(domain.Product{
					ID:           1,
					Name:         "iPhone 15",
					Price:        599900,
					Stock:        100,
					Categories:   []string{"电子产品", "手机"},
					MerchantID:   1001,
					MerchantName: "Apple Store",
				}, nil)
				return svc
			},
			wantID:   1,
			wantName: "iPhone 15",
			wantErr:  false,
		},
		{
			name: "商品不存在",
			req:  &v1.GetProductReq{Id: 999},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().GetProduct(gomock.Any(), int64(999)).Return(domain.Product{}, errors.New("not found"))
				return svc
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewProductHandler(svc)

			resp, err := h.GetProduct(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, resp.Product.Id)
				assert.Equal(t, tc.wantName, resp.Product.Name)
			}
		})
	}
}

func TestProductHandler_CreateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		req     *v1.CreateProductReq
		mock    func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantID  int64
		wantErr bool
	}{
		{
			name: "成功创建商品",
			req: &v1.CreateProductReq{
				Product: &v1.Product{
					Name:       "新商品",
					Price:      9900,
					Stock:      50,
					Categories: []string{"服装"},
					MerchantID: 1001,
				},
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().CreateProduct(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				return svc
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "创建失败",
			req: &v1.CreateProductReq{
				Product: &v1.Product{
					Name:  "敏感商品",
					Price: 9900,
				},
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().CreateProduct(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("sensitive word"))
				return svc
			},
			wantID:  0,
			wantErr: true,
		},
		{
			name: "空请求体",
			req:  &v1.CreateProductReq{Product: nil},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				// 空 Product 会被转换为空 domain.Product
				svc.EXPECT().CreateProduct(gomock.Any(), domain.Product{}).Return(int64(0), errors.New("invalid product"))
				return svc
			},
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewProductHandler(svc)

			resp, err := h.CreateProduct(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, resp.Id)
			}
		})
	}
}

func TestProductHandler_UpdateProduct(t *testing.T) {
	testCases := []struct {
		name    string
		req     *v1.UpdateProductReq
		mock    func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantID  int64
		wantErr bool
	}{
		{
			name: "成功更新商品",
			req: &v1.UpdateProductReq{
				Product: &v1.Product{
					Id:    1,
					Name:  "更新后的商品",
					Price: 19900,
				},
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().UpdateProduct(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				return svc
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "更新失败",
			req: &v1.UpdateProductReq{
				Product: &v1.Product{
					Id:   999,
					Name: "不存在的商品",
				},
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().UpdateProduct(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("not found"))
				return svc
			},
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewProductHandler(svc)

			resp, err := h.UpdateProduct(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, resp.Id)
			}
		})
	}
}

func TestProductHandler_DeleteProduct(t *testing.T) {
	testCases := []struct {
		name    string
		req     *v1.DeleteProductReq
		mock    func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantErr bool
	}{
		{
			name: "成功删除商品",
			req: &v1.DeleteProductReq{
				Id:     1,
				UserId: 1001,
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(1001)).Return(nil)
				return svc
			},
			wantErr: false,
		},
		{
			name: "删除失败-无权限",
			req: &v1.DeleteProductReq{
				Id:     1,
				UserId: 9999,
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(9999)).Return(errors.New("no permission"))
				return svc
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewProductHandler(svc)

			resp, err := h.DeleteProduct(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

// ============================================================================
// 转换函数测试
// ============================================================================

func TestToProto(t *testing.T) {
	product := domain.Product{
		ID:           1,
		Name:         "iPhone 15",
		Description:  "最新款 iPhone",
		Picture:      "http://example.com/iphone.jpg",
		SlideImgs:    []string{"http://example.com/1.jpg", "http://example.com/2.jpg"},
		Price:        599900,
		Categories:   []string{"电子产品", "手机"},
		Stock:        100,
		MerchantID:   1001,
		MerchantName: "Apple Store",
	}

	proto := toProto(product)

	assert.Equal(t, int64(1), proto.Id)
	assert.Equal(t, "iPhone 15", proto.Name)
	assert.Equal(t, "最新款 iPhone", proto.Description)
	assert.Equal(t, "http://example.com/iphone.jpg", proto.Picture)
	assert.Equal(t, []string{"http://example.com/1.jpg", "http://example.com/2.jpg"}, proto.SliderImgs)
	assert.Equal(t, int64(599900), proto.Price)
	assert.Equal(t, []string{"电子产品", "手机"}, proto.Categories)
	assert.Equal(t, int64(100), proto.Stock)
	assert.Equal(t, int64(1001), proto.MerchantID)
	assert.Equal(t, "Apple Store", proto.MerchantName)
}

func TestToDomain(t *testing.T) {
	proto := &v1.Product{
		Id:           1,
		Name:         "iPhone 15",
		Description:  "最新款 iPhone",
		Picture:      "http://example.com/iphone.jpg",
		SliderImgs:   []string{"http://example.com/1.jpg"},
		Price:        599900,
		Categories:   []string{"电子产品"},
		Stock:        100,
		MerchantID:   1001,
		MerchantName: "Apple Store",
	}

	domain := toDomain(proto)

	assert.Equal(t, int64(1), domain.ID)
	assert.Equal(t, "iPhone 15", domain.Name)
	assert.Equal(t, int64(599900), domain.Price)
}

func TestToDomain_Nil(t *testing.T) {
	result := toDomain(nil)
	assert.Equal(t, domain.Product{}, result)
}

func TestToProtoList(t *testing.T) {
	products := []domain.Product{
		{ID: 1, Name: "商品1"},
		{ID: 2, Name: "商品2"},
		{ID: 3, Name: "商品3"},
	}

	protos := toProtoList(products)

	assert.Len(t, protos, 3)
	assert.Equal(t, int64(1), protos[0].Id)
	assert.Equal(t, int64(2), protos[1].Id)
	assert.Equal(t, int64(3), protos[2].Id)
}

