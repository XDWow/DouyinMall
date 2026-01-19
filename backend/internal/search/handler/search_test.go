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

// MockSearchService � service.SearchService � mock 实现
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

// 注意：SyncProduct 和 BatchSyncProducts 的测试已删除
// 这些 RPC 方法已被移除，数据同步现在通过内部 Kafka 消费者处理
func _Deleted_TestSearchHandler_SyncProduct(t *testing.T) {
	testCases := []struct {
		name        string
		req         *searchv1.SyncProductReq
		mock        func() (*MockSearchService, *MockSyncService)
		wantSuccess bool
		wantErr     bool
	}{
		{
			name: "成功同步商品",
			req: &searchv1.SyncProductReq{
				Action: searchv1.SyncAction_CREATE,
				Product: &searchv1.ProductDocument{
					Id:    1,
					Name:  "iPhone 15",
					Price: 599900,
				},
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				syncSvc := &MockSyncService{
					SyncFunc: func(ctx context.Context, event domain.SyncEvent) error {
						assert.Equal(t, domain.EventTypeProduct, event.Type)
						assert.Equal(t, domain.EventActionCreate, event.Action)
						assert.Equal(t, int64(1), event.ID)
						assert.NotNil(t, event.Product)
						return nil
					},
				}
				return &MockSearchService{}, syncSvc
			},
			wantSuccess: true,
			wantErr:     false,
		},
		{
			name: "同步失败",
			req: &searchv1.SyncProductReq{
				Action: searchv1.SyncAction_UPDATE,
				Product: &searchv1.ProductDocument{
					Id:   1,
					Name: "iPhone 15 Pro",
				},
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				syncSvc := &MockSyncService{
					SyncFunc: func(ctx context.Context, event domain.SyncEvent) error {
						return errors.New("ES sync error")
					},
				}
				return &MockSearchService{}, syncSvc
			},
			wantSuccess: false,
			wantErr:     false, // Handler 不返回错误，而是返回 Success=false 的响应
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searchSvc, syncSvc := tc.mock()
			h := NewSearchHandler(searchSvc, syncSvc, nil, nil)

			// resp, err := h.SyncProduct(context.Background(), tc.req)
			// 方法已删除
		})
	}
}

func _Deleted_TestSearchHandler_BatchSyncProducts(t *testing.T) {
	testCases := []struct {
		name        string
		req         *searchv1.BatchSyncProductsReq
		mock        func() (*MockSearchService, *MockSyncService)
		wantSuccess int64
		wantFailed  int64
		wantErr     bool
	}{
		{
			name: "批量同步成功",
			req: &searchv1.BatchSyncProductsReq{
				Products: []*searchv1.ProductDocument{
					{Id: 1, Name: "iPhone 15"},
					{Id: 2, Name: "iPhone 14"},
				},
			},
			mock: func() (*MockSearchService, *MockSyncService) {
				syncSvc := &MockSyncService{
					BatchSyncFunc: func(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string) {
						assert.Len(t, events, 2)
						return 2, 0, nil
					},
				}
				return &MockSearchService{}, syncSvc
			},
			wantSuccess: 2,
			wantFailed:  0,
			wantErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searchSvc, syncSvc := tc.mock()
			h := NewSearchHandler(searchSvc, syncSvc, nil, nil)

			// resp, err := h.BatchSyncProducts(context.Background(), tc.req)
			// 方法已删除
		})
	}
}
