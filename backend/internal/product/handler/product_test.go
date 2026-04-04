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
			name: "鎴愬姛鑾峰彇鍟嗗搧鍒楄〃",
			req: &v1.ListProductsReq{
				Page:     1,
				PageSize: 10,
				Category: "",
			},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "").Return([]domain.Product{
					{ID: 1, Name: "鍟嗗搧1", Price: 100},
					{ID: 2, Name: "鍟嗗搧2", Price: 200},
				}, nil)
				return svc
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "Service杩斿洖閿欒",
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

func TestProductHandler_GetProducts(t *testing.T) {
	testCases := []struct {
		name     string
		req      *v1.GetProductsReq
		mock     func(ctrl *gomock.Controller) *svcmocks.MockProductService
		wantID   int64
		wantName string
		wantErr  bool
	}{
		{
			name: "鎴愬姛鑾峰彇鍟嗗搧璇︽儏",
			req:  &v1.GetProductsReq{Id: []int64{1}},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				svc.EXPECT().GetProduct(gomock.Any(), int64(1)).Return(domain.Product{
					ID:           1,
					Name:         "iPhone 15",
					Price:        599900,
					InStock:      true, // domain 灞備娇鐢?InStock (bool)
					Categories:   []string{"鐢靛瓙浜у搧", "鎵嬫満"},
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
			name: "鍟嗗搧涓嶅瓨鍦?,
			req:  &v1.GetProductsReq{Id: []int64{999}},
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

			resp, err := h.GetProducts(context.Background(), tc.req)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, resp.Product[0].Id)
				assert.Equal(t, tc.wantName, resp.Product[0].Name)
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
			name: "鎴愬姛鍒涘缓鍟嗗搧",
			req: &v1.CreateProductReq{
				Product: &v1.Product{
					Name:       "鏂板晢鍝?,
					Price:      9900,
					InStock:    true, // 鏈夎揣
					Categories: []string{"鏈嶈"},
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
			name: "鍒涘缓澶辫触",
			req: &v1.CreateProductReq{
				Product: &v1.Product{
					Name:  "鏁忔劅鍟嗗搧",
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
			name: "绌鸿姹備綋",
			req:  &v1.CreateProductReq{Product: nil},
			mock: func(ctrl *gomock.Controller) *svcmocks.MockProductService {
				svc := svcmocks.NewMockProductService(ctrl)
				// 绌?Product 浼氳杞崲涓虹┖ domain.Product
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
			name: "鎴愬姛鏇存柊鍟嗗搧",
			req: &v1.UpdateProductReq{
				Product: &v1.Product{
					Id:    1,
					Name:  "鏇存柊鍚庣殑鍟嗗搧",
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
			name: "鏇存柊澶辫触",
			req: &v1.UpdateProductReq{
				Product: &v1.Product{
					Id:   999,
					Name: "涓嶅瓨鍦ㄧ殑鍟嗗搧",
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
			name: "鎴愬姛鍒犻櫎鍟嗗搧",
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
			name: "鍒犻櫎澶辫触-鏃犳潈闄?,
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
// 杞崲鍑芥暟娴嬭瘯
// ============================================================================

func TestToProto(t *testing.T) {
	product := domain.Product{
		ID:           1,
		Name:         "iPhone 15",
		Description:  "鏈€鏂版 iPhone",
		Picture:      "http://example.com/iphone.jpg",
		SlideImgs:    []string{"http://example.com/1.jpg", "http://example.com/2.jpg"},
		Price:        599900,
		Categories:   []string{"鐢靛瓙浜у搧", "鎵嬫満"},
		InStock:      true, // domain 灞備娇鐢?InStock (bool)
		MerchantID:   1001,
		MerchantName: "Apple Store",
	}

	proto := toProto(product)

	assert.Equal(t, int64(1), proto.Id)
	assert.Equal(t, "iPhone 15", proto.Name)
	assert.Equal(t, "鏈€鏂版 iPhone", proto.Description)
	assert.Equal(t, "http://example.com/iphone.jpg", proto.Picture)
	assert.Equal(t, []string{"http://example.com/1.jpg", "http://example.com/2.jpg"}, proto.SliderImgs)
	assert.Equal(t, int64(599900), proto.Price)
	assert.Equal(t, []string{"鐢靛瓙浜у搧", "鎵嬫満"}, proto.Categories)
	assert.Equal(t, true, proto.InStock) // stock=100 > 0锛屾墍浠?in_stock 搴旇涓?true
	assert.Equal(t, int64(1001), proto.MerchantID)
	assert.Equal(t, "Apple Store", proto.MerchantName)
}

func TestToDomain(t *testing.T) {
	proto := &v1.Product{
		Id:           1,
		Name:         "iPhone 15",
		Description:  "鏈€鏂版 iPhone",
		Picture:      "http://example.com/iphone.jpg",
		SliderImgs:   []string{"http://example.com/1.jpg"},
		Price:        599900,
		Categories:   []string{"鐢靛瓙浜у搧"},
		InStock:      true, // 鏈夎揣
		MerchantID:   1001,
		MerchantName: "Apple Store",
	}

	domain := toDomain(proto)

	assert.Equal(t, int64(1), domain.ID)
	assert.Equal(t, "iPhone 15", domain.Name)
	assert.Equal(t, int64(599900), domain.Price)
	assert.Equal(t, true, domain.InStock) // 鐩存帴浣跨敤 in_stock
}

func TestToDomain_Nil(t *testing.T) {
	result := toDomain(nil)
	assert.Equal(t, domain.Product{}, result)
}

func TestToProtoList(t *testing.T) {
	products := []domain.Product{
		{ID: 1, Name: "鍟嗗搧1"},
		{ID: 2, Name: "鍟嗗搧2"},
		{ID: 3, Name: "鍟嗗搧3"},
	}

	protos := toProtoList(products)

	assert.Len(t, protos, 3)
	assert.Equal(t, int64(1), protos[0].Id)
	assert.Equal(t, int64(2), protos[1].Id)
	assert.Equal(t, int64(3), protos[2].Id)
}


