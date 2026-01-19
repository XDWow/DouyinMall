package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
)

type ProductRepo interface {
	SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error)
	SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)
	GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error)

	// 索引管理
	SyncProduct(ctx context.Context, action string, doc *domain.ProductDocument) error
	BatchSyncProducts(ctx context.Context, docs []domain.ProductDocument) (successCount, failedCount int64, errors []string)
	DeleteProduct(ctx context.Context, productID int64) error
	BatchDeleteProducts(ctx context.Context, productIDs []int64) (successCount, failedCount int64, errors []string)
}

type MerchantRepo interface {
	SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error)
	SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)

	// 索引管理
	SyncMerchant(ctx context.Context, action string, doc *domain.MerchantDocument) error
	BatchSyncMerchants(ctx context.Context, docs []domain.MerchantDocument) (successCount, failedCount int64, errors []string)
	DeleteMerchant(ctx context.Context, merchantID int64) error
	BatchDeleteMerchants(ctx context.Context, merchantIDs []int64) (successCount, failedCount int64, errors []string)
}
