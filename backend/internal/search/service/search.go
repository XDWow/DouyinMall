package service

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type SearchService interface {
	SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error)
	SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error)
	SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)
	SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)
	GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error)
}

type searchService struct {
	productRepo  repo.ProductRepo
	merchantRepo repo.MerchantRepo
	logger       logger.LoggerV1
}

func NewSearchService(productRepo repo.ProductRepo, merchantRepo repo.MerchantRepo, logger logger.LoggerV1) SearchService {
	return &searchService{
		productRepo:  productRepo,
		merchantRepo: merchantRepo,
		logger:       logger,
	}
}

func (s *searchService) SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
	return s.productRepo.SearchProducts(ctx, req)
}

func (s *searchService) SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error) {
	return s.merchantRepo.SearchMerchants(ctx, req)
}

func (s *searchService) SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return s.productRepo.SearchProductSuggest(ctx, keyword, limit)
}

func (s *searchService) SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return s.merchantRepo.SearchMerchantSuggest(ctx, keyword, limit)
}

func (s *searchService) GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	return s.productRepo.GetAggregations(ctx, req)
}
