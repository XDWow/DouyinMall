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
	// 敏感词校验
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("产品包含敏感词，创建失败")
		return 0, err
	}
	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 上传一下 照片，轮播图
	}
	return svc.repo.CreateProduct(ctx, product)
}

func (svc *productService) UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error) {
	// 敏感词校验（只校验非空字段）
	if err := svc.checkSensitiveWords(ctx, product.Name, product.Description); err != nil {
		svc.logger.Error("产品包含敏感词，创建失败")
		return 0, err
	}

	if len(product.Picture) != 0 || len(product.SlideImgs) != 0 {
		// 更新照片，轮播图
	}
	return svc.repo.UpdateProduct(ctx, product)
}

func (svc *productService) DeleteProduct(ctx context.Context, id, userID int64) (err error) {
	return svc.repo.DeleteProduct(ctx, id, userID)
}

func (svc *productService) checkSensitiveWords(ctx context.Context, productName string, productDescription string) error {
	// 后面搞 ai 检测
	if strings.ContainsAny(productName, "敏感词") || strings.ContainsAny(productDescription, "敏感词") {
		return fmt.Errorf("产品名字或描述存在敏感词汇！")
	}
	return nil
}
