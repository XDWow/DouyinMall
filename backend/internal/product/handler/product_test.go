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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().ListProducts(gomock.Any(), int64(1), int64(10), "phone").Return([]domain.Product{
		{ID: 1, Name: "Phone", Price: 100, Currency: "CNY"},
		{ID: 2, Name: "Case", Price: 20, Currency: "CNY"},
	}, nil)

	h := NewProductHandler(svc)
	resp, err := h.ListProducts(context.Background(), &v1.ListProductsReq{
		Page:     1,
		PageSize: 10,
		Category: "phone",
	})

	require.NoError(t, err)
	require.Len(t, resp.GetProducts(), 2)
	assert.Equal(t, int64(1), resp.GetProducts()[0].GetId())
}

func TestProductHandler_GetProducts_UsesProductAndSKU(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().
		GetProducts(gomock.Any(), []domain.ProductQuery{{ID: 1, SKUID: 101}}).
		Return([]domain.Product{{
			ID:       1,
			SKUID:    101,
			Name:     "iPhone 15",
			Price:    599900,
			Currency: "CNY",
			InStock:  true,
		}}, nil)

	h := NewProductHandler(svc)
	resp, err := h.GetProducts(context.Background(), &v1.GetProductsReq{
		Items: []*v1.ProductQuery{{ProductId: 1, SkuId: 101}},
	})

	require.NoError(t, err)
	require.Len(t, resp.GetProduct(), 1)
	assert.Equal(t, int64(1), resp.GetProduct()[0].GetId())
	assert.Equal(t, int64(101), resp.GetProduct()[0].GetSkuId())
	assert.Equal(t, "iPhone 15", resp.GetProduct()[0].GetName())
}

func TestProductHandler_GetProducts_ReturnsServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().
		GetProducts(gomock.Any(), []domain.ProductQuery{{ID: 999, SKUID: 100999}}).
		Return(nil, errors.New("not found"))

	h := NewProductHandler(svc)
	resp, err := h.GetProducts(context.Background(), &v1.GetProductsReq{
		Items: []*v1.ProductQuery{{ProductId: 999, SkuId: 100999}},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestProductHandler_GetProductQuotes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().
		GetProductQuotes(gomock.Any(), []domain.ProductQuery{{ID: 1, SKUID: 101}}).
		Return([]domain.ProductQuote{{
			ProductID: 1,
			SKUID:     101,
			Price:     599900,
			Currency:  "CNY",
			InStock:   true,
		}}, nil)

	h := NewProductHandler(svc)
	resp, err := h.GetProductQuotes(context.Background(), &v1.GetProductQuotesReq{
		Items: []*v1.ProductQuery{{ProductId: 1, SkuId: 101}},
	})

	require.NoError(t, err)
	require.Len(t, resp.GetProductQuotes(), 1)
	assert.Equal(t, int64(1), resp.GetProductQuotes()[0].GetProductId())
	assert.Equal(t, int64(101), resp.GetProductQuotes()[0].GetSkuId())
	assert.Equal(t, int64(599900), resp.GetProductQuotes()[0].GetPrice())
}

func TestProductHandler_CreateProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().CreateProduct(gomock.Any(), domain.Product{
		SKUID:      101,
		Name:       "Phone",
		Price:      9900,
		Currency:   "CNY",
		Categories: []string{"electronics"},
		InStock:    true,
		MerchantID: 1001,
	}).Return(int64(1), nil)

	h := NewProductHandler(svc)
	resp, err := h.CreateProduct(context.Background(), &v1.CreateProductReq{
		Product: &v1.Product{
			SkuId:      101,
			Name:       "Phone",
			Price:      9900,
			Currency:   "CNY",
			Categories: []string{"electronics"},
			InStock:    true,
			MerchantID: 1001,
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.GetId())
}

func TestProductHandler_UpdateProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().UpdateProduct(gomock.Any(), domain.Product{
		ID:         1,
		SKUID:      101,
		Name:       "Phone Pro",
		Price:      12900,
		Currency:   "CNY",
		InStock:    true,
		MerchantID: 1001,
	}).Return(int64(1), nil)

	h := NewProductHandler(svc)
	resp, err := h.UpdateProduct(context.Background(), &v1.UpdateProductReq{
		Product: &v1.Product{
			Id:         1,
			SkuId:      101,
			Name:       "Phone Pro",
			Price:      12900,
			Currency:   "CNY",
			InStock:    true,
			MerchantID: 1001,
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.GetId())
}

func TestProductHandler_DeleteProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := svcmocks.NewMockProductService(ctrl)
	svc.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(1001)).Return(nil)

	h := NewProductHandler(svc)
	resp, err := h.DeleteProduct(context.Background(), &v1.DeleteProductReq{
		Id:     1,
		UserId: 1001,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestToProtoAndDomain(t *testing.T) {
	product := domain.Product{
		ID:           1,
		SKUID:        101,
		Name:         "iPhone 15",
		Description:  "Apple phone",
		Picture:      "http://example.com/iphone.jpg",
		SlideImgs:    []string{"http://example.com/1.jpg"},
		Price:        599900,
		Currency:     "CNY",
		Categories:   []string{"electronics"},
		InStock:      true,
		MerchantID:   1001,
		MerchantName: "Apple Store",
	}

	proto := toProto(product)
	assert.Equal(t, int64(101), proto.GetSkuId())
	assert.Equal(t, "CNY", proto.GetCurrency())

	back := toDomain(proto)
	assert.Equal(t, product, back)
	assert.Equal(t, domain.Product{}, toDomain(nil))
}
