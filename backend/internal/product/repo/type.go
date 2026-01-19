package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
)

type ProductRepo interface {
	ListProducts(ctx context.Context, page, pageSize int64, category string) (products []domain.Product, err error)
	GetProduct(ctx context.Context, id int64) (product domain.Product, err error)
	CreateProduct(ctx context.Context, product domain.Product) (productID int64, err error)
	UpdateProduct(ctx context.Context, product domain.Product) (productID int64, err error)
	DeleteProduct(ctx context.Context, id, userID int64) (err error)
}
