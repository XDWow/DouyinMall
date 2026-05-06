package grpc

import (
	"context"
	"fmt"
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/XDWow/DouyinMall/backend/internal/search/usecase"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1"
)

// SearchHandler 閹兼粎鍌ㄩ張宥呭 gRPC handler
type SearchHandler struct {
	searchProductsUC  *usecase.SearchProductsUseCase
	searchMerchantsUC *usecase.SearchMerchantsUseCase
	suggestUC         *usecase.SuggestUseCase
	aggregationsUC    *usecase.AggregationsUseCase
	aiSearchUC        *usecase.AISearchUseCase
	syncUC            *usecase.SyncUseCase

	esClient      *es.ESClient
	productClient productservice.Client
}

func NewSearchHandler(
	searchProductsUC *usecase.SearchProductsUseCase,
	searchMerchantsUC *usecase.SearchMerchantsUseCase,
	suggestUC *usecase.SuggestUseCase,
	aggregationsUC *usecase.AggregationsUseCase,
	aiSearchUC *usecase.AISearchUseCase,
	syncUC *usecase.SyncUseCase,
	esClient *es.ESClient,
	productClient productservice.Client,
) *SearchHandler {
	return &SearchHandler{
		searchProductsUC:  searchProductsUC,
		searchMerchantsUC: searchMerchantsUC,
		suggestUC:         suggestUC,
		aggregationsUC:    aggregationsUC,
		aiSearchUC:        aiSearchUC,
		syncUC:            syncUC,
		esClient:          esClient,
		productClient:     productClient,
	}
}

func (h *SearchHandler) SearchProducts(ctx context.Context, req *searchv1.SearchProductsReq) (*searchv1.SearchProductsResp, error) {
	resp, err := h.searchProductsUC.Execute(ctx, toDomainSearchProductsReq(req))
	if err != nil {
		return nil, err
	}
	return &searchv1.SearchProductsResp{
		Products: toProtoProductList(resp.Products),
		Total:    resp.Total, Page: resp.Page, PageSize: resp.PageSize,
	}, nil
}

func (h *SearchHandler) SearchMerchants(ctx context.Context, req *searchv1.SearchMerchantsReq) (*searchv1.SearchMerchantsResp, error) {
	resp, err := h.searchMerchantsUC.Execute(ctx, toDomainSearchMerchantsReq(req))
	if err != nil {
		return nil, err
	}
	return &searchv1.SearchMerchantsResp{
		Merchants: toProtoMerchantList(resp.Merchants),
		Total:     resp.Total, Page: resp.Page, PageSize: resp.PageSize,
	}, nil
}

func (h *SearchHandler) SearchSuggest(ctx context.Context, req *searchv1.SearchSuggestReq) (*searchv1.SearchSuggestResp, error) {
	var suggestions []domain.SearchSuggestion
	var err error
	switch req.GetType() {
	case searchv1.SuggestType_MERCHANT:
		suggestions, err = h.suggestUC.MerchantSuggest(ctx, req.GetKeyword(), req.GetLimit())
	default:
		suggestions, err = h.suggestUC.ProductSuggest(ctx, req.GetKeyword(), req.GetLimit())
	}
	if err != nil {
		return nil, err
	}
	return &searchv1.SearchSuggestResp{Suggestions: toProtoSuggestionList(suggestions)}, nil
}

func (h *SearchHandler) GetSearchAggregations(ctx context.Context, req *searchv1.SearchProductsReq) (*searchv1.SearchAggregationsResp, error) {
	resp, err := h.aggregationsUC.Execute(ctx, toDomainSearchProductsReq(req))
	if err != nil {
		return nil, err
	}
	return &searchv1.SearchAggregationsResp{
		Categories:  toProtoCategoryAggList(resp.Categories),
		PriceRanges: toProtoPriceRangeAggList(resp.PriceRanges),
	}, nil
}

func (h *SearchHandler) AISearchProducts(ctx context.Context, req *searchv1.AISearchProductsReq) (*searchv1.AISearchProductsResp, error) {
	resp, err := h.aiSearchUC.Execute(ctx, &domain.AISearchProductsReq{
		Query:           req.GetQuery(),
		Page:            req.GetPage(),
		PageSize:        req.GetPageSize(),
		EnableRAG:       req.GetEnableRag(),
		EnableHighlight: req.GetEnableHighlight(),
	})
	if err != nil {
		return nil, err
	}

	result := &searchv1.AISearchProductsResp{
		Products: toProtoProductList(resp.Products),
		Total:    resp.Total, Page: resp.Page, PageSize: resp.PageSize,
		RagSummary: resp.RAGSummary,
	}
	if resp.QueryIntent != nil {
		result.QueryIntent = &searchv1.QueryIntent{
			RewrittenQuery: resp.QueryIntent.RewrittenQuery,
			Categories:     resp.QueryIntent.Categories,
			MinPrice:       resp.QueryIntent.MinPrice,
			MaxPrice:       resp.QueryIntent.MaxPrice,
			SortBy:         resp.QueryIntent.SortBy,
			Intent:         resp.QueryIntent.Intent,
			NeedRag:        resp.QueryIntent.NeedRAG,
		}
	}
	if resp.Metrics != nil {
		result.Metrics = &searchv1.PipelineMetrics{
			QueryUnderstandingMs: resp.Metrics.QueryUnderstandingMs,
			KeywordRecallMs:      resp.Metrics.KeywordRecallMs,
			VectorRecallMs:       resp.Metrics.VectorRecallMs,
			RankingMs:            resp.Metrics.RankingMs,
			FetchMs:              resp.Metrics.FetchMs,
			RagMs:                resp.Metrics.RAGMs,
			TotalMs:              resp.Metrics.TotalMs,
			KeywordRecallCount:   resp.Metrics.KeywordRecallCount,
			VectorRecallCount:    resp.Metrics.VectorRecallCount,
		}
	}
	return result, nil
}

func (h *SearchHandler) InitES(ctx context.Context, req *searchv1.InitESReq) (*searchv1.InitESResp, error) {
	resp := &searchv1.InitESResp{Success: true, Message: "init es completed", IndicesCreated: true}

	if req.GetRecreateIndices() {
		if err := es.InitIndices(h.esClient); err != nil {
			return &searchv1.InitESResp{Success: false, Message: fmt.Sprintf("init indices failed: %v", err)}, nil
		}
		resp.Message = "indices recreated"
	}

	if req.GetSyncProducts() {
		batchSize := req.GetBatchSize()
		if batchSize <= 0 {
			batchSize = 1000
		}
		log.Printf("sync products to es with batch size: %d", batchSize)
		s, f, errs := h.syncProductsFromRPC(ctx, batchSize)
		resp.ProductsSynced = s
		resp.ProductsFailed = f
		resp.Errors = append(resp.Errors, errs...)
		if f > 0 {
			resp.Success = false
			resp.Message = fmt.Sprintf("sync products finished with failures: success %d, failed %d", s, f)
		}
	}

	if req.GetSyncMerchants() {
		resp.Errors = append(resp.Errors, "merchant sync is not implemented yet")
	}

	return resp, nil
}

// syncProductsFromRPC 娴?Product Service 閹峰褰囬崗銊╁櫤閺佺増宓侀崥灞绢劄閸?ES
func (h *SearchHandler) syncProductsFromRPC(ctx context.Context, batchSize int64) (success, failed int64, errors []string) {
	page := int64(1)
	for {
		resp, err := h.productClient.ListProducts(ctx, &productv1.ListProductsReq{
			Page: page, PageSize: batchSize,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("閹峰褰囩粭?%d 妞ら潧銇戠拹? %v", page, err))
			break
		}
		products := resp.GetProducts()
		if len(products) == 0 {
			break
		}

		events := make([]domain.SyncEvent, 0, len(products))
		for _, p := range products {
			doc := productProtoToDoc(p)
			events = append(events, domain.SyncEvent{
				Type: domain.EventTypeProduct, Action: domain.EventActionCreate,
				ID: doc.ID, Product: &doc,
			})
		}

		s, f, errs := h.syncUC.BatchSync(ctx, events)
		success += s
		failed += f
		errors = append(errors, errs...)

		if int64(len(products)) < batchSize {
			break
		}
		page++
	}
	return
}

func productProtoToDoc(p *productv1.Product) domain.ProductDocument {
	return domain.ProductDocument{
		ID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(),
		Picture: p.GetPicture(), SliderImgs: p.GetSliderImgs(),
		Price: p.GetPrice(), Categories: p.GetCategories(),
		InStock: p.GetInStock(), MerchantID: p.GetMerchantID(),
		MerchantName: p.GetMerchantName(),
	}
}
