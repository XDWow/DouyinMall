package domain

import "context"

// ProductRepo 鍟嗗搧鎼滅储浠撳偍鎺ュ彛锛坉omain 澹版槑鎵€闇€绔彛锛宨nfra 鎻愪緵瀹炵幇锛?
type ProductRepo interface {
	SearchProducts(ctx context.Context, req *SearchProductsReq) (*SearchProductsResp, error)
	SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]SearchSuggestion, error)
	GetAggregations(ctx context.Context, req *SearchProductsReq) (*SearchAggregationsResp, error)

	// AI 鎼滅储
	VectorSearch(ctx context.Context, vector []float32, topK int64, filters map[string]interface{}) ([]RecallResult, error)
	KeywordRecallSearch(ctx context.Context, query string) ([]RecallResult, error)
	GetProductsByIDs(ctx context.Context, ids []int64, enableHighlight bool, keyword string) ([]ProductSearchResult, error)

	// 绱㈠紩绠＄悊
	SyncProduct(ctx context.Context, action string, doc *ProductDocument) error
	BatchSyncProducts(ctx context.Context, docs []ProductDocument) (success, failed int64, errors []string)
	DeleteProduct(ctx context.Context, productID int64) error
	BatchDeleteProducts(ctx context.Context, productIDs []int64) (success, failed int64, errors []string)
}

// MerchantRepo 鍟嗗鎼滅储浠撳偍鎺ュ彛
type MerchantRepo interface {
	SearchMerchants(ctx context.Context, req *SearchMerchantsReq) (*SearchMerchantsResp, error)
	SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]SearchSuggestion, error)

	SyncMerchant(ctx context.Context, action string, doc *MerchantDocument) error
	BatchSyncMerchants(ctx context.Context, docs []MerchantDocument) (success, failed int64, errors []string)
	DeleteMerchant(ctx context.Context, merchantID int64) error
	BatchDeleteMerchants(ctx context.Context, merchantIDs []int64) (success, failed int64, errors []string)
}


