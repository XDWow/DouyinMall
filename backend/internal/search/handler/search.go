package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo/es"
	"github.com/XDWow/DouyinMall/backend/internal/search/service"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1"
)

type SearchHandler struct {
	searchSvc service.SearchService
	syncSvc   service.SyncService
	esClient  *es.ESClient // 用于初始化索引

	// 批量同步数据
	productClient productservice.Client
	// TODO: Merchant Service RPC 客户端

}

func NewSearchHandler(
	searchSvc service.SearchService,
	syncSvc service.SyncService,
	esClient *es.ESClient,
	productClient productservice.Client,
) *SearchHandler {
	return &SearchHandler{
		searchSvc:     searchSvc,
		syncSvc:       syncSvc,
		esClient:      esClient,
		productClient: productClient,
	}
}

func (h *SearchHandler) SearchProducts(ctx context.Context, req *searchv1.SearchProductsReq) (*searchv1.SearchProductsResp, error) {
	domainReq := toDomainSearchProductsReq(req)

	resp, err := h.searchSvc.SearchProducts(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	return &searchv1.SearchProductsResp{
		Products: toProtoProductList(resp.Products),
		Total:    resp.Total,
		Page:     resp.Page,
		PageSize: resp.PageSize,
	}, nil
}

func (h *SearchHandler) SearchMerchants(ctx context.Context, req *searchv1.SearchMerchantsReq) (*searchv1.SearchMerchantsResp, error) {
	domainReq := toDomainSearchMerchantsReq(req)

	resp, err := h.searchSvc.SearchMerchants(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	return &searchv1.SearchMerchantsResp{
		Merchants: toProtoMerchantList(resp.Merchants),
		Total:     resp.Total,
		Page:      resp.Page,
		PageSize:  resp.PageSize,
	}, nil
}

func (h *SearchHandler) SearchSuggest(ctx context.Context, req *searchv1.SearchSuggestReq) (*searchv1.SearchSuggestResp, error) {
	var suggestions []domain.SearchSuggestion
	var err error

	switch req.GetType() {
	case searchv1.SuggestType_MERCHANT:
		suggestions, err = h.searchSvc.SearchMerchantSuggest(ctx, req.GetKeyword(), req.GetLimit())
	default:
		suggestions, err = h.searchSvc.SearchProductSuggest(ctx, req.GetKeyword(), req.GetLimit())
	}

	if err != nil {
		return nil, err
	}

	return &searchv1.SearchSuggestResp{
		Suggestions: toProtoSuggestionList(suggestions),
	}, nil
}

func (h *SearchHandler) GetSearchAggregations(ctx context.Context, req *searchv1.SearchProductsReq) (*searchv1.SearchAggregationsResp, error) {
	domainReq := toDomainSearchProductsReq(req)

	resp, err := h.searchSvc.GetAggregations(ctx, domainReq)
	if err != nil {
		return nil, err
	}

	return &searchv1.SearchAggregationsResp{
		Categories:  toProtoCategoryAggList(resp.Categories),
		PriceRanges: toProtoPriceRangeAggList(resp.PriceRanges),
	}, nil
}

func (h *SearchHandler) InitES(ctx context.Context, req *searchv1.InitESReq) (*searchv1.InitESResp, error) {
	resp := &searchv1.InitESResp{
		Success:        true,
		Message:        "操作成功",
		IndicesCreated: true,
	}

	if req.GetRecreateIndices() {
		// 目前索引创建是幂等的，已存在的索引不会重复创建
		// 如果需要重建，需要先删除索引，然后调用 InitIndices
		err := es.InitIndices(h.esClient)
		if err != nil {
			return &searchv1.InitESResp{
				Success:        false,
				Message:        fmt.Sprintf("重建索引失败: %v", err),
				IndicesCreated: false,
			}, nil
		}
		resp.Message = "索引已重建"
	}

	// 批量同步数据：从 Product Service 拉取全量数据
	if req.GetSyncProducts() {
		batchSize := req.GetBatchSize()
		if batchSize <= 0 {
			batchSize = 1000
		}

		log.Printf("开始从 Product Service 同步商品数据，批次大小: %d", batchSize)
		successCount, failedCount, errors := h.syncProductsFromRPC(ctx, batchSize)
		resp.ProductsSynced = successCount
		resp.ProductsFailed = failedCount
		resp.Errors = append(resp.Errors, errors...)

		if failedCount > 0 {
			resp.Success = false
			resp.Message = fmt.Sprintf("部分商品同步失败（成功: %d, 失败: %d）", successCount, failedCount)
		} else {
			log.Printf("商品数据同步完成，成功: %d 条", successCount)
		}
	}

	if req.GetSyncMerchants() {
		resp.Errors = append(resp.Errors, "商家数据同步暂未实现")
	}

	if resp.ProductsFailed > 0 || resp.MerchantsFailed > 0 {
		resp.Success = false
		resp.Message = fmt.Sprintf("索引创建成功，但部分数据同步失败（商品: %d/%d, 商家: %d/%d）",
			resp.ProductsFailed, resp.ProductsSynced+resp.ProductsFailed,
			resp.MerchantsFailed, resp.MerchantsSynced+resp.MerchantsFailed)
	}

	return resp, nil
}

// 注意：同步相关的 RPC 方法已移除
// 现在数据同步直接通过内部 Kafka 消费者处理，不需要 RPC 调用

func toProtoProductList(products []domain.ProductSearchResult) []*searchv1.ProductSearchResult {
	res := make([]*searchv1.ProductSearchResult, len(products))
	for i, p := range products {
		res[i] = &searchv1.ProductSearchResult{
			Id:                   p.ID,
			Name:                 p.Name,
			Description:          p.Description,
			Picture:              p.Picture,
			SliderImgs:           p.SliderImgs,
			Price:                p.Price,
			Categories:           p.Categories,
			InStock:              p.InStock, // 是否有货
			MerchantId:           p.MerchantID,
			MerchantName:         p.MerchantName,
			Score:                p.Score,
			NameHighlight:        p.NameHighlight,
			DescriptionHighlight: p.DescriptionHighlight,
		}
	}
	return res
}

func toProtoMerchantList(merchants []domain.MerchantSearchResult) []*searchv1.MerchantSearchResult {
	res := make([]*searchv1.MerchantSearchResult, len(merchants))
	for i, m := range merchants {
		res[i] = &searchv1.MerchantSearchResult{
			Id:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			Logo:          m.Logo,
			Region:        m.Region,
			Rating:        m.Rating,
			SalesCount:    m.SalesCount,
			ProductCount:  m.ProductCount,
			Verified:      m.Verified,
			Score:         m.Score,
			NameHighlight: m.NameHighlight,
		}
	}
	return res
}

func toProtoSuggestionList(suggestions []domain.SearchSuggestion) []*searchv1.SearchSuggestion {
	res := make([]*searchv1.SearchSuggestion, len(suggestions))
	for i, s := range suggestions {
		var source searchv1.SuggestSource
		switch s.Source {
		case "HISTORY":
			source = searchv1.SuggestSource_HISTORY
		case "HOT":
			source = searchv1.SuggestSource_HOT
		case "NAME_MATCH":
			source = searchv1.SuggestSource_NAME_MATCH
		default:
			source = searchv1.SuggestSource_NAME_MATCH
		}

		res[i] = &searchv1.SearchSuggestion{
			Keyword: s.Keyword,
			Source:  source,
			Count:   s.Count,
		}
	}
	return res
}

func toProtoCategoryAggList(aggs []domain.CategoryAggregation) []*searchv1.CategoryAggregation {
	res := make([]*searchv1.CategoryAggregation, len(aggs))
	for i, a := range aggs {
		res[i] = &searchv1.CategoryAggregation{
			Category: a.Category,
			Count:    a.Count,
		}
	}
	return res
}

func toProtoPriceRangeAggList(aggs []domain.PriceRangeAggregation) []*searchv1.PriceRangeAggregation {
	res := make([]*searchv1.PriceRangeAggregation, len(aggs))
	for i, a := range aggs {
		res[i] = &searchv1.PriceRangeAggregation{
			MinPrice: a.MinPrice,
			MaxPrice: a.MaxPrice,
			Count:    a.Count,
			Label:    a.Label,
		}
	}
	return res
}

func toDomainSearchProductsReq(req *searchv1.SearchProductsReq) *domain.SearchProductsReq {
	return &domain.SearchProductsReq{
		Keyword:         req.GetKeyword(),
		Page:            req.GetPage(),
		PageSize:        req.GetPageSize(),
		Categories:      req.GetCategories(),
		MinPrice:        req.GetMinPrice(),
		MaxPrice:        req.GetMaxPrice(),
		MerchantID:      req.GetMerchantId(),
		InStockOnly:     true, // 只看有货
		SortBy:          req.GetSortBy().String(),
		EnableHighlight: req.GetEnableHighlight(),
	}
}

func toDomainSearchMerchantsReq(req *searchv1.SearchMerchantsReq) *domain.SearchMerchantsReq {
	verified := req.GetVerifiedOnly()
	return &domain.SearchMerchantsReq{
		Keyword:   req.GetKeyword(),
		Page:      req.GetPage(),
		PageSize:  req.GetPageSize(),
		Region:    req.GetRegion(),
		MinRating: req.GetMinRating(),
		Verified:  &verified,
		SortBy:    req.GetSortBy().String(),
	}
}

// 从 Product Service 分页拉取全量商品数据并同步到 ES
func (h *SearchHandler) syncProductsFromRPC(ctx context.Context, batchSize int64) (successCount, failedCount int64, errors []string) {
	page := int64(1)
	totalSynced := int64(0)

	for {
		resp, err := h.productClient.ListProducts(ctx, &productv1.ListProductsReq{
			Page:     page,
			PageSize: batchSize,
			Category: "",
		})
		if err != nil {
			errMsg := fmt.Sprintf("从 Product Service 拉取第 %d 页数据失败: %v", page, err)
			errors = append(errors, errMsg)
			log.Println(errMsg)
			break
		}

		products := resp.GetProducts()
		if len(products) == 0 {
			log.Printf("数据拉取完成，共 %d 页，总计 %d 条", page-1, totalSynced)
			break
		}

		events := make([]domain.SyncEvent, 0, len(products))
		for _, p := range products {
			doc := productProtoToDomainDocument(p)
			events = append(events, domain.SyncEvent{
				Type:    domain.EventTypeProduct,
				Action:  domain.EventActionCreate, // 全量同步全是插入
				ID:      doc.ID,
				Product: &doc,
			})
		}

		success, failed, errs := h.syncSvc.BatchSync(ctx, events)
		successCount += success
		failedCount += failed
		errors = append(errors, errs...)

		totalSynced += int64(len(products))
		log.Printf("第 %d 页同步完成：成功 %d 条，失败 %d 条，累计 %d 条", page, success, failed, totalSynced)

		// 如果返回的数据少于批次大小，说明已经是最后一页
		if int64(len(products)) < batchSize {
			log.Printf("数据拉取完成，共 %d 页，总计 %d 条", page, totalSynced)
			break
		}

		page++
	}

	return successCount, failedCount, errors
}

func productProtoToDomainDocument(p *productv1.Product) domain.ProductDocument {
	return domain.ProductDocument{
		ID:           p.GetId(),
		Name:         p.GetName(),
		Description:  p.GetDescription(),
		Picture:      p.GetPicture(),
		SliderImgs:   p.GetSliderImgs(),
		Price:        p.GetPrice(),
		Categories:   p.GetCategories(),
		InStock:      p.GetInStock(),
		MerchantID:   p.GetMerchantID(),
		MerchantName: p.GetMerchantName(),
		// CreatedTime 和 UpdatedTime 在 Product proto 中没有，使用默认值 0
		CreatedTime: 0,
		UpdatedTime: 0,
	}
}
