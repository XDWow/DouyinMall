package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// SearchProductsUseCase 普通商品搜索（关键词 + 筛选 + 排序）
type SearchProductsUseCase struct {
	productRepo domain.ProductRepo
	l           logger.LoggerV1
}

func NewSearchProductsUseCase(productRepo domain.ProductRepo, l logger.LoggerV1) *SearchProductsUseCase {
	return &SearchProductsUseCase{productRepo: productRepo, l: l}
}

func (uc *SearchProductsUseCase) Execute(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
	return uc.productRepo.SearchProducts(ctx, req)
}

// SearchMerchantsUseCase 商家搜索
type SearchMerchantsUseCase struct {
	merchantRepo domain.MerchantRepo
	l            logger.LoggerV1
}

func NewSearchMerchantsUseCase(merchantRepo domain.MerchantRepo, l logger.LoggerV1) *SearchMerchantsUseCase {
	return &SearchMerchantsUseCase{merchantRepo: merchantRepo, l: l}
}

func (uc *SearchMerchantsUseCase) Execute(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error) {
	return uc.merchantRepo.SearchMerchants(ctx, req)
}

// SuggestUseCase 搜索建议（自动补全）
type SuggestUseCase struct {
	productRepo  domain.ProductRepo
	merchantRepo domain.MerchantRepo
}

func NewSuggestUseCase(productRepo domain.ProductRepo, merchantRepo domain.MerchantRepo) *SuggestUseCase {
	return &SuggestUseCase{productRepo: productRepo, merchantRepo: merchantRepo}
}

func (uc *SuggestUseCase) ProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return uc.productRepo.SearchProductSuggest(ctx, keyword, limit)
}

func (uc *SuggestUseCase) MerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return uc.merchantRepo.SearchMerchantSuggest(ctx, keyword, limit)
}

// AggregationsUseCase 聚合统计
type AggregationsUseCase struct {
	productRepo domain.ProductRepo
}

func NewAggregationsUseCase(productRepo domain.ProductRepo) *AggregationsUseCase {
	return &AggregationsUseCase{productRepo: productRepo}
}

func (uc *AggregationsUseCase) Execute(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	return uc.productRepo.GetAggregations(ctx, req)
}
