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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().ListProducts(gomock.Any(), int64(1), int64(20), "").Return([]domain.Product{
		{ID: 1, Name: "Phone"},
	}, nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	products, err := svc.ListProducts(context.Background(), 0, 0, "")

	require.NoError(t, err)
	require.Len(t, products, 1)
}

func TestProductService_GetProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	query := domain.ProductQuery{ID: 1, SKUID: 101}
	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().GetProduct(gomock.Any(), query).Return(domain.Product{
		ID:    1,
		SKUID: 101,
		Name:  "Phone",
	}, nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	product, err := svc.GetProduct(context.Background(), query)

	require.NoError(t, err)
	assert.Equal(t, int64(101), product.SKUID)
}

func TestProductService_GetProducts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	queries := []domain.ProductQuery{
		{ID: 1, SKUID: 101},
		{ID: 2, SKUID: 202},
	}
	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().GetProducts(gomock.Any(), queries).Return([]domain.Product{
		{ID: 1, SKUID: 101, Name: "Phone"},
		{ID: 2, SKUID: 202, Name: "Case"},
	}, nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	products, err := svc.GetProducts(context.Background(), queries)

	require.NoError(t, err)
	require.Len(t, products, 2)
	assert.Equal(t, int64(202), products[1].SKUID)
}

func TestProductService_GetProductQuotes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	queries := []domain.ProductQuery{
		{ID: 1, SKUID: 101},
		{ID: 2, SKUID: 202},
	}
	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().GetProductQuotes(gomock.Any(), queries).Return([]domain.ProductQuote{
		{ProductID: 1, SKUID: 101, Price: 100, Currency: "CNY", InStock: true},
		{ProductID: 2, SKUID: 202, Price: 200, Currency: "CNY", InStock: false},
	}, nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	quotes, err := svc.GetProductQuotes(context.Background(), queries)

	require.NoError(t, err)
	require.Len(t, quotes, 2)
	assert.Equal(t, int64(202), quotes[1].SKUID)
}

func TestProductService_GetProduct_RequiresSKU(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewProductService(repomocks.NewMockProductRepo(ctrl), logger.NewNopLogger())
	product, err := svc.GetProduct(context.Background(), domain.ProductQuery{ID: 1})

	assert.Error(t, err)
	assert.Equal(t, domain.Product{}, product)
}

func TestProductService_GetProducts_RequiresSKU(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewProductService(repomocks.NewMockProductRepo(ctrl), logger.NewNopLogger())
	products, err := svc.GetProducts(context.Background(), []domain.ProductQuery{{ID: 1}})

	assert.Error(t, err)
	assert.Nil(t, products)
}

func TestProductService_CreateProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	product := validProduct()
	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().CreateProduct(gomock.Any(), product).Return(int64(1), nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	id, err := svc.CreateProduct(context.Background(), product)

	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestProductService_CreateProduct_ValidationAndSensitiveWords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewProductService(repomocks.NewMockProductRepo(ctrl), logger.NewNopLogger())

	_, err := svc.CreateProduct(context.Background(), domain.Product{Name: "missing sku", MerchantID: 1})
	assert.Error(t, err)

	product := validProduct()
	product.Name = "blocked item"
	_, err = svc.CreateProduct(context.Background(), product)
	assert.Error(t, err)
}

func TestProductService_UpdateProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	product := validProduct()
	product.ID = 1
	product.Name = "Phone Pro"

	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().UpdateProduct(gomock.Any(), product).Return(int64(1), nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	id, err := svc.UpdateProduct(context.Background(), product)

	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestProductService_UpdateProduct_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	product := validProduct()
	product.ID = 1

	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().UpdateProduct(gomock.Any(), product).Return(int64(0), errors.New("db error"))

	svc := NewProductService(repo, logger.NewNopLogger())
	_, err := svc.UpdateProduct(context.Background(), product)

	assert.Error(t, err)
}

func TestProductService_DeleteProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomocks.NewMockProductRepo(ctrl)
	repo.EXPECT().DeleteProduct(gomock.Any(), int64(1), int64(1001)).Return(nil)

	svc := NewProductService(repo, logger.NewNopLogger())
	err := svc.DeleteProduct(context.Background(), 1, 1001)

	require.NoError(t, err)
}

func validProduct() domain.Product {
	return domain.Product{
		SKUID:      101,
		Name:       "Phone",
		Price:      9900,
		Currency:   "CNY",
		InStock:    true,
		MerchantID: 1001,
	}
}
