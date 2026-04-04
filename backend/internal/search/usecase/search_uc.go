package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// SearchProductsUseCase 鏅€氬晢鍝佹悳绱紙鍏抽敭璇?+ 绛涢€?+ 鎺掑簭锛?
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

// SearchMerchantsUseCase 鍟嗗鎼滅储
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

// SuggestUseCase 鎼滅储寤鸿锛堣嚜鍔ㄨˉ鍏級
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

// AggregationsUseCase 鑱氬悎缁熻
type AggregationsUseCase struct {
	productRepo domain.ProductRepo
}

func NewAggregationsUseCase(productRepo domain.ProductRepo) *AggregationsUseCase {
	return &AggregationsUseCase{productRepo: productRepo}
}

func (uc *AggregationsUseCase) Execute(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	return uc.productRepo.GetAggregations(ctx, req)
}


