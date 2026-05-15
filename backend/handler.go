package backend

import (
	"context"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1"
)

// SearchServiceImpl implements the last service interface defined in the IDL.
type SearchServiceImpl struct{}

// SearchProducts implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) SearchProducts(ctx context.Context, req *v1.SearchProductsReq) (resp *v1.SearchProductsResp, err error) {
	// TODO: Your code here...
	return
}

// GetSearchAggregations implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) GetSearchAggregations(ctx context.Context, req *v1.SearchProductsReq) (resp *v1.SearchAggregationsResp, err error) {
	// TODO: Your code here...
	return
}

// SearchMerchants implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) SearchMerchants(ctx context.Context, req *v1.SearchMerchantsReq) (resp *v1.SearchMerchantsResp, err error) {
	// TODO: Your code here...
	return
}

// SearchSuggest implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) SearchSuggest(ctx context.Context, req *v1.SearchSuggestReq) (resp *v1.SearchSuggestResp, err error) {
	// TODO: Your code here...
	return
}

// InitES implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) InitES(ctx context.Context, req *v1.InitESReq) (resp *v1.InitESResp, err error) {
	// TODO: Your code here...
	return
}

// AISearchProducts implements the SearchServiceImpl interface.
func (s *SearchServiceImpl) AISearchProducts(ctx context.Context, req *v1.AISearchProductsReq) (resp *v1.AISearchProductsResp, err error) {
	// TODO: Your code here...
	return
}
