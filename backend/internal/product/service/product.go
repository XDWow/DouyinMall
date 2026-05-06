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
	// 闁轰礁绻戦崝鍛嫚瀹ュ棛澧″Δ?
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("sensitive word validation failed", logger.Error(err))
		return 0, err
	}
	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 濞戞挸锕ｇ槐鑸电▔閳ь剚绋?闁绘挆鍛暬闁挎稑鐭侀悿鍡涘箻椤撶偞绂?
	}
	return svc.repo.CreateProduct(ctx, product)
}

func (svc *productService) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	// 闁轰礁绻戦崝鍛嫚瀹ュ棛澧″Δ鐘茬焿缁辨瑩宕ｉ鍛ⅰ濡ょ姴鐭傚顏嗙矚閸濆嫮鎽熸繛鍫㈩暜缁?
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("sensitive word validation failed", logger.Error(err))
		return 0, err
	}

	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 闁哄洤鐡ㄩ弻濠囨偂瑜忔晶鏍晬瀹€鍐瀭闁圭虎鍘煎ù?
	}
	return svc.repo.UpdateProduct(ctx, product)
}

func (svc *productService) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	return svc.repo.DeleteProduct(ctx, id, userID)
}

func (svc *productService) checkSensitiveWords(ctx context.Context, productName string, productDescription string) error {
	// 闁告艾閰ｅ浼村箹?ai 婵☆偀鍋撴繛?
	if strings.Contains(productName, "blocked") || strings.Contains(productDescription, "blocked") {
		return fmt.Errorf("product contains sensitive words")
	}
	return nil
}
