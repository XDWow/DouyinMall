package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockSearchService struct {
	SearchProductsFunc        func(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error)
	SearchMerchantsFunc       func(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error)
	SearchProductSuggestFunc  func(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)
	SearchMerchantSuggestFunc func(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error)
	GetAggregationsFunc       func(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error)
}

func (m *MockSearchService) SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
	return m.SearchProductsFunc(ctx, req)
}

func (m *MockSearchService) SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error) {
	return m.SearchMerchantsFunc(ctx, req)
}

func (m *MockSearchService) SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return m.SearchProductSuggestFunc(ctx, keyword, limit)
}

func (m *MockSearchService) SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	return m.SearchMerchantSuggestFunc(ctx, keyword, limit)
}

func (m *MockSearchService) GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	return m.GetAggregationsFunc(ctx, req)
}

// MockSyncService � service.SyncService � mock 实现
type MockSyncService struct {
	SyncFunc      func(ctx context.Context, event domain.SyncEvent) error
	BatchSyncFunc func(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string)
}

func (m *MockSyncService) Sync(ctx context.Context, event domain.SyncEvent) error {
	return m.SyncFunc(ctx, event)
}

func (m *MockSyncService) BatchSync(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string) {
	return m.BatchSyncFunc(ctx, events)
}

func TestSearchHandler_SearchProducts(t *testing.T) {
	testCases := []struct {
		name      string
		req       *searchv1.SearchProductsReq
		mock      func() (*MockSearchService, *MockSyncService)
		wantCount int
		wantErr   bool
	}{
		{
			name: "成功搜索商品",
			req: &searchv1.SearchProductsReq{
				Keyword:  "iPhone",
				Page:     1,
				PageSize: 10,
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				searchSvc := &MockSearchService{
					SearchProductsFunc: func(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
						assert.Equal(t, "iPhone", req.Keyword)
						return &domain.SearchProductsResp{
							Products: []domain.ProductSearchResult{
								{ID: 1, Name: "iPhone 15", Price: 599900},
								{ID: 2, Name: "iPhone 14", Price: 499900},
							},
							Total:    2,
							Page:     1,
							PageSize: 10,
						}, nil
					},
				}
				return searchSvc, &MockSyncService{}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "Service返回错误",
			req: &searchv1.SearchProductsReq{
				Keyword:  "iPhone",
				Page:     1,
				PageSize: 10,
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				searchSvc := &MockSearchService{
					SearchProductsFunc: func(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
						return nil, errors.New("ES error")
					},
				}
				return searchSvc, &MockSyncService{}
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searchSvc, syncSvc := tc.mock()
			h := NewSearchHandler(searchSvc, syncSvc, nil, nil)

			resp, err := h.SearchProducts(context.Background(), tc.req)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Len(t, resp.Products, tc.wantCount)
				assert.Equal(t, int64(2), resp.Total)
			}
		})
	}
}

func TestSearchHandler_SearchSuggest(t *testing.T) {
	testCases := []struct {
		name      string
		req       *searchv1.SearchSuggestReq
		mock      func() (*MockSearchService, *MockSyncService)
		wantCount int
		wantErr   bool
	}{
		{
			name: "商品搜索建议",
			req: &searchv1.SearchSuggestReq{
				Keyword: "iPh",
				Limit:   10,
				Type:    searchv1.SuggestType_PRODUCT,
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				searchSvc := &MockSearchService{
					SearchProductSuggestFunc: func(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
						assert.Equal(t, "iPh", keyword)
						assert.Equal(t, int64(10), limit)
						return []domain.SearchSuggestion{
							{Keyword: "iPhone 15", Source: "NAME_MATCH"},
							{Keyword: "iPhone 14", Source: "NAME_MATCH"},
						}, nil
					},
				}
				return searchSvc, &MockSyncService{}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "商家搜索建议",
			req: &searchv1.SearchSuggestReq{
				Keyword: "Apple",
				Limit:   10,
				Type:    searchv1.SuggestType_MERCHANT,
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				searchSvc := &MockSearchService{
					SearchMerchantSuggestFunc: func(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
						assert.Equal(t, "Apple", keyword)
						return []domain.SearchSuggestion{
							{Keyword: "Apple Store", Source: "NAME_MATCH"},
						}, nil
					},
				}
				return searchSvc, &MockSyncService{}
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searchSvc, syncSvc := tc.mock()
			h := NewSearchHandler(searchSvc, syncSvc, nil, nil)

			resp, err := h.SearchSuggest(context.Background(), tc.req)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, resp.Suggestions, tc.wantCount)
			}
		})
	}
}
