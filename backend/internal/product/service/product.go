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
	GetProduct(ctx context.Context, id int64) (product domain.Product, err error)
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

func (svc *productService) ListProducts(ctx context.Context, page, pageSize int64, category string) (products []domain.Product, err error) {
	return svc.repo.ListProducts(ctx, page, pageSize, category)
}

func (svc *productService) GetProduct(ctx context.Context, id int64) (product domain.Product, err error) {
	return svc.repo.GetProduct(ctx, id)
}

func (svc *productService) CreateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	// 鏁忔劅璇嶆牎楠?
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("浜у搧鍖呭惈鏁忔劅璇嶏紝鍒涘缓澶辫触")
		return 0, err
	}
	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 涓婁紶涓€涓?鐓х墖锛岃疆鎾浘
	}
	return svc.repo.CreateProduct(ctx, product)
}

func (svc *productService) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	// 鏁忔劅璇嶆牎楠岋紙鍙牎楠岄潪绌哄瓧娈碉級
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("浜у搧鍖呭惈鏁忔劅璇嶏紝鍒涘缓澶辫触")
		return 0, err
	}

	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 鏇存柊鐓х墖锛岃疆鎾浘
	}
	return svc.repo.UpdateProduct(ctx, product)
}

func (svc *productService) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	return svc.repo.DeleteProduct(ctx, id, userID)
}

func (svc *productService) checkSensitiveWords(ctx context.Context, productName string, productDescription string) error {
	// 鍚庨潰鎼?ai 妫€娴?
	if strings.ContainsAny(productName, "鏁忔劅璇?) || strings.ContainsAny(productDescription, "鏁忔劅璇?) {
		return fmt.Errorf("浜у搧鍚嶅瓧鎴栨弿杩板瓨鍦ㄦ晱鎰熻瘝姹囷紒")
	}
	return nil
}


