package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ProductService interface {
	ListProducts(ctx context.Context, page, pageSize int64, category string) (products []domain.Product, err error)
	GetProduct(ctx context.Context, query domain.ProductQuery) (product domain.Product, err error)
	CreateProduct(ctx context.Context, product domain.Product) (productID int64, err error)
	UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error)
	DeleteProduct(ctx context.Context, id, userID int64) (err error)
}

type productService struct {
	repo   repo.ProductRepo
	logger logger.LoggerV1
}

func NewProductService(repo repo.ProductRepo, logger logger.LoggerV1) ProductService {
	return &productService{
		repo:   repo,
		logger: logger,
	}
}

func (svc *productService) ListProducts(ctx context.Context, page, pageSize int64, category string) ([]domain.Product, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return svc.repo.ListProducts(ctx, page, pageSize, category)
}

func (svc *productService) GetProduct(ctx context.Context, query domain.ProductQuery) (domain.Product, error) {
	if err := validateProductQuery(query); err != nil {
		return domain.Product{}, err
	}
	return svc.repo.GetProduct(ctx, query)
}

func (svc *productService) CreateProduct(ctx context.Context, product domain.Product) (int64, error) {
	if err := validateProductForWrite(product, false); err != nil {
		return 0, err
	}
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("sensitive word validation failed", logger.Error(err))
		return 0, err
	}
	return svc.repo.CreateProduct(ctx, product)
}

func (svc *productService) UpdateProduct(ctx context.Context, product domain.Product) (int64, error) {
	if err := validateProductForWrite(product, true); err != nil {
		return 0, err
	}
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("sensitive word validation failed", logger.Error(err))
		return 0, err
	}
	return svc.repo.UpdateProduct(ctx, product)
}

func (svc *productService) DeleteProduct(ctx context.Context, id, userID int64) error {
	if id <= 0 {
		return fmt.Errorf("product_id is required")
	}
	if userID <= 0 {
		return fmt.Errorf("user_id is required")
	}
	return svc.repo.DeleteProduct(ctx, id, userID)
}

func (svc *productService) checkSensitiveWords(ctx context.Context, productName string, productDescription string) error {
	if strings.Contains(productName, "blocked") || strings.Contains(productDescription, "blocked") {
		return fmt.Errorf("product contains sensitive words")
	}
	return nil
}

func validateProductQuery(query domain.ProductQuery) error {
	if query.ID <= 0 {
		return fmt.Errorf("product_id is required")
	}
	if query.SKUID <= 0 {
		return fmt.Errorf("sku_id is required")
	}
	return nil
}

func validateProductForWrite(product domain.Product, requireID bool) error {
	if requireID && product.ID <= 0 {
		return fmt.Errorf("product_id is required")
	}
	if strings.TrimSpace(product.Name) == "" {
		return fmt.Errorf("product name is required")
	}
	if product.SKUID <= 0 {
		return fmt.Errorf("sku_id is required")
	}
	if product.Price < 0 {
		return fmt.Errorf("price must be non-negative")
	}
	if product.MerchantID <= 0 {
		return fmt.Errorf("merchant_id is required")
	}
	return nil
}
