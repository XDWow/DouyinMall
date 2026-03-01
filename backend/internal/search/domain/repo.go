package domain

import "context"

// ProductRepo 商品搜索仓储接口（domain 声明所需端口，infra 提供实现）
type ProductRepo interface {
	SearchProducts(ctx context.Context, req *SearchProductsReq) (*SearchProductsResp, error)
	SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]SearchSuggestion, error)
	GetAggregations(ctx context.Context, req *SearchProductsReq) (*SearchAggregationsResp, error)

	// AI 搜索
	VectorSearch(ctx context.Context, vector []float32, topK int64, filters map[string]interface{}) ([]RecallResult, error)
	KeywordRecallSearch(ctx context.Context, query string) ([]RecallResult, error)
	GetProductsByIDs(ctx context.Context, ids []int64, enableHighlight bool, keyword string) ([]ProductSearchResult, error)

	// 索引管理
	SyncProduct(ctx context.Context, action string, doc *ProductDocument) error
	BatchSyncProducts(ctx context.Context, docs []ProductDocument) (success, failed int64, errors []string)
	DeleteProduct(ctx context.Context, productID int64) error
	BatchDeleteProducts(ctx context.Context, productIDs []int64) (success, failed int64, errors []string)
}

// MerchantRepo 商家搜索仓储接口
type MerchantRepo interface {
	SearchMerchants(ctx context.Context, req *SearchMerchantsReq) (*SearchMerchantsResp, error)
	SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]SearchSuggestion, error)

	SyncMerchant(ctx context.Context, action string, doc *MerchantDocument) error
	BatchSyncMerchants(ctx context.Context, docs []MerchantDocument) (success, failed int64, errors []string)
	DeleteMerchant(ctx context.Context, merchantID int64) error
	BatchDeleteMerchants(ctx context.Context, merchantIDs []int64) (success, failed int64, errors []string)
}
