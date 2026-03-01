package es

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type productRepo struct {
	es *ESClient
	l  logger.LoggerV1
}

// NewProductRepo 创建商品仓储（实现 domain.ProductRepo 端口）
func NewProductRepo(esClient *ESClient, l logger.LoggerV1) domain.ProductRepo {
	return &productRepo{es: esClient, l: l}
}

func (r *productRepo) SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
	start := time.Now()
	query := r.buildProductQuery(req)
	res, err := r.es.Search(ctx, ProductIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result esSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	products := make([]domain.ProductSearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		products = append(products, hitToProduct(hit, req.EnableHighlight))
	}

	r.l.Info("商品搜索完成",
		logger.Int64("took_ms", time.Since(start).Milliseconds()),
		logger.Int64("total", result.Hits.Total.Value))

	return &domain.SearchProductsResp{
		Products: products, Total: result.Hits.Total.Value,
		Page: req.Page, PageSize: req.PageSize,
	}, nil
}

// VectorSearch ES8 kNN 向量搜索
func (r *productRepo) VectorSearch(ctx context.Context, vector []float32, topK int64, filters map[string]interface{}) ([]domain.RecallResult, error) {
	knnQuery := map[string]interface{}{
		"knn": map[string]interface{}{
			"field":          "name_vector",
			"query_vector":   vector,
			"k":              topK,
			"num_candidates": topK * 2,
		},
		"size":    topK,
		"_source": []string{"id", "sales_count"},
	}
	if len(filters) > 0 {
		knnQuery["knn"].(map[string]interface{})["filter"] = filters
	}

	queryJSON, _ := json.Marshal(knnQuery)
	res, err := r.es.KnnSearch(ctx, ProductIndex, string(queryJSON))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result esSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 kNN 结果失败: %w", err)
	}

	recalls := make([]domain.RecallResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		recalls = append(recalls, domain.RecallResult{
			ProductID:  hit.Source.ID,
			Score:      hit.Score,
			Source:     domain.RecallVector,
			SalesCount: hit.Source.SalesCount,
		})
	}
	return recalls, nil
}

func (r *productRepo) SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	suggestions, err := r.es.SearchSuggest(ctx, ProductIndex, keyword, limit)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SearchSuggestion, 0, len(suggestions))
	for _, sug := range suggestions {
		if text, ok := sug["text"].(string); ok {
			s := domain.SearchSuggestion{Keyword: text, Source: "NAME_MATCH"}
			if score, ok := sug["score"].(float64); ok {
				s.Count = int64(score)
			}
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *productRepo) GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	query := r.buildAggregationQuery(req)
	res, err := r.es.Search(ctx, ProductIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result esAggResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析聚合结果失败: %w", err)
	}

	categories := make([]domain.CategoryAggregation, 0, len(result.Aggs.CategoryAgg.Buckets))
	for _, b := range result.Aggs.CategoryAgg.Buckets {
		categories = append(categories, domain.CategoryAggregation{Category: b.Key, Count: b.Count})
	}

	priceRanges := make([]domain.PriceRangeAggregation, 0, len(result.Aggs.PriceRangeAgg.Buckets))
	for _, b := range result.Aggs.PriceRangeAgg.Buckets {
		minP := parseESPrice(b.From)
		maxP := parseESPrice(b.To)
		priceRanges = append(priceRanges, domain.PriceRangeAggregation{
			MinPrice: minP, MaxPrice: maxP,
			Count: b.Count, Label: formatPriceLabel(minP, maxP),
		})
	}

	return &domain.SearchAggregationsResp{Categories: categories, PriceRanges: priceRanges}, nil
}

// GetProductsByIDs 通过 ES ids 查询精确获取指定商品（用于 AI 搜索第四阶段）
func (r *productRepo) GetProductsByIDs(ctx context.Context, ids []int64, enableHighlight bool, keyword string) ([]domain.ProductSearchResult, error) {
	strIDs := make([]interface{}, len(ids))
	for i, id := range ids {
		strIDs[i] = strconv.FormatInt(id, 10)
	}
	queryObj := map[string]interface{}{
		"size": len(ids),
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{"ids": map[string]interface{}{"values": strIDs}},
				},
			},
		},
	}
	// 如果提供了关键词且开启高亮，将 keyword 作为 should 子句（不影响过滤，只用于高亮）
	if enableHighlight && keyword != "" {
		queryObj["query"].(map[string]interface{})["bool"].(map[string]interface{})["should"] = []interface{}{
			map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":  keyword,
					"fields": []string{"name^3", "description"},
				},
			},
		}
		queryObj["highlight"] = map[string]interface{}{
			"fields":    map[string]interface{}{"name": map[string]interface{}{}, "description": map[string]interface{}{}},
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
		}
	}
	queryJSON, _ := json.Marshal(queryObj)
	res, err := r.es.Search(ctx, ProductIndex, string(queryJSON))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result esSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 IDs 查询结果失败: %w", err)
	}
	products := make([]domain.ProductSearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		products = append(products, hitToProduct(hit, enableHighlight))
	}
	return products, nil
}

// ==================== 索引管理 ====================

func (r *productRepo) SyncProduct(ctx context.Context, action string, doc *domain.ProductDocument) error {
	docID := strconv.FormatInt(doc.ID, 10)
	switch action {
	case "CREATE":
		return r.es.IndexDocument(ctx, ProductIndex, docID, doc)
	case "UPDATE":
		return r.es.UpdateDocument(ctx, ProductIndex, docID, doc)
	case "DELETE":
		return r.es.DeleteDocument(ctx, ProductIndex, docID)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}
}

func (r *productRepo) BatchSyncProducts(ctx context.Context, docs []domain.ProductDocument) (int64, int64, []string) {
	docMaps := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		docMaps = append(docMaps, map[string]interface{}{
			"id": doc.ID, "name": doc.Name, "description": doc.Description,
			"picture": doc.Picture, "slider_imgs": doc.SliderImgs,
			"price": doc.Price, "categories": doc.Categories,
			"in_stock": doc.InStock, "merchant_id": doc.MerchantID,
			"merchant_name": doc.MerchantName, "sales_count": doc.SalesCount,
			"name_vector": doc.NameVector,
			"created_at":  doc.CreatedTime, "updated_at": doc.UpdatedTime,
		})
	}
	if err := r.es.BulkIndex(ctx, ProductIndex, docMaps); err != nil {
		return 0, int64(len(docs)), []string{err.Error()}
	}
	return int64(len(docs)), 0, nil
}

func (r *productRepo) DeleteProduct(ctx context.Context, productID int64) error {
	return r.es.DeleteDocument(ctx, ProductIndex, strconv.FormatInt(productID, 10))
}

func (r *productRepo) BatchDeleteProducts(ctx context.Context, ids []int64) (int64, int64, []string) {
	if len(ids) == 0 {
		return 0, 0, nil
	}
	docIDs := make([]string, len(ids))
	for i, id := range ids {
		docIDs[i] = strconv.FormatInt(id, 10)
	}
	if err := r.es.BulkDelete(ctx, ProductIndex, docIDs); err != nil {
		return 0, int64(len(ids)), []string{err.Error()}
	}
	return int64(len(ids)), 0, nil
}

// ==================== 查询构建 ====================

func (r *productRepo) buildProductQuery(req *domain.SearchProductsReq) string {
	queryObj := map[string]interface{}{
		"from": (req.Page - 1) * req.PageSize,
		"size": req.PageSize,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   []interface{}{},
				"filter": []interface{}{},
			},
		},
		"sort": []interface{}{},
	}

	boolQ := queryObj["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := boolQ["must"].([]interface{})
	filter := boolQ["filter"].([]interface{})
	sort := queryObj["sort"].([]interface{})

	if req.Keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  req.Keyword,
				"fields": []string{"name^3", "description"},
			},
		})
	}
	if len(req.Categories) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{"categories": req.Categories},
		})
	}
	if req.MinPrice > 0 || req.MaxPrice > 0 {
		pr := map[string]interface{}{}
		if req.MinPrice > 0 {
			pr["gte"] = req.MinPrice
		}
		if req.MaxPrice > 0 {
			pr["lte"] = req.MaxPrice
		}
		filter = append(filter, map[string]interface{}{"range": map[string]interface{}{"price": pr}})
	}
	if req.MerchantID > 0 {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"merchant_id": req.MerchantID}})
	}
	if req.InStockOnly {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"in_stock": true}})
	}

	switch req.SortBy {
	case "PRICE_ASC":
		sort = append(sort, map[string]interface{}{"price": "asc"})
	case "PRICE_DESC":
		sort = append(sort, map[string]interface{}{"price": "desc"})
	case "SALES_DESC":
		sort = append(sort, map[string]interface{}{"sales_count": "desc"})
	case "NEW_ARRIVAL":
		sort = append(sort, map[string]interface{}{"created_at": "desc"})
	default:
		sort = append(sort, map[string]interface{}{"_score": "desc"})
	}

	if req.EnableHighlight {
		queryObj["highlight"] = map[string]interface{}{
			"fields":    map[string]interface{}{"name": map[string]interface{}{}, "description": map[string]interface{}{}},
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
		}
	}

	boolQ["must"] = must
	boolQ["filter"] = filter
	queryObj["sort"] = sort
	queryJSON, _ := json.Marshal(queryObj)
	return string(queryJSON)
}

func (r *productRepo) buildAggregationQuery(req *domain.SearchProductsReq) string {
	baseQuery := r.buildProductQuery(req)
	var queryObj map[string]interface{}
	json.Unmarshal([]byte(baseQuery), &queryObj)
	queryObj["size"] = 0
	queryObj["aggs"] = map[string]interface{}{
		"category_agg": map[string]interface{}{
			"terms": map[string]interface{}{"field": "categories", "size": 100},
		},
		"price_range_agg": map[string]interface{}{
			"range": map[string]interface{}{
				"field": "price",
				"ranges": []map[string]interface{}{
					{"key": "0-50", "to": 5000},
					{"key": "50-100", "from": 5000, "to": 10000},
					{"key": "100-200", "from": 10000, "to": 20000},
					{"key": "200-500", "from": 20000, "to": 50000},
					{"key": "500-1000", "from": 50000, "to": 100000},
					{"key": "1000+", "from": 100000},
				},
			},
		},
	}
	queryJSON, _ := json.Marshal(queryObj)
	return string(queryJSON)
}

// ==================== AI 搜索辅助函数 ====================

// BuildFiltersFromIntent 根据 QueryIntent 构建 ES 过滤条件
func BuildFiltersFromIntent(intent *domain.QueryIntent) map[string]interface{} {
	var filterList []interface{}
	if len(intent.Categories) > 0 {
		filterList = append(filterList, map[string]interface{}{
			"terms": map[string]interface{}{"categories": intent.Categories},
		})
	}
	if intent.MinPrice > 0 || intent.MaxPrice > 0 {
		pr := map[string]interface{}{}
		if intent.MinPrice > 0 {
			pr["gte"] = intent.MinPrice
		}
		if intent.MaxPrice > 0 {
			pr["lte"] = intent.MaxPrice
		}
		filterList = append(filterList, map[string]interface{}{
			"range": map[string]interface{}{"price": pr},
		})
	}
	filterList = append(filterList, map[string]interface{}{
		"term": map[string]interface{}{"in_stock": true},
	})
	return map[string]interface{}{
		"bool": map[string]interface{}{"filter": filterList},
	}
}

// BuildKeywordQuery 构建关键词召回 query（仅返回 id + sales_count，供融合排序使用）
func BuildKeywordQuery(keyword string, topK int64, filters map[string]interface{}) string {
	must := []interface{}{
		map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  keyword,
				"fields": []string{"name^3", "description"},
			},
		},
	}
	boolQ := map[string]interface{}{"must": must}
	if f, ok := filters["bool"].(map[string]interface{})["filter"]; ok {
		boolQ["filter"] = f
	}
	queryObj := map[string]interface{}{
		"size":    topK,
		"_source": []string{"id", "sales_count"},
		"query":   map[string]interface{}{"bool": boolQ},
	}
	body, _ := json.Marshal(queryObj)
	return string(body)
}

// KeywordRecallSearch 执行关键词召回搜索
func (r *productRepo) KeywordRecallSearch(ctx context.Context, query string) ([]domain.RecallResult, error) {
	res, err := r.es.Search(ctx, ProductIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result esSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析关键词召回结果失败: %w", err)
	}

	recalls := make([]domain.RecallResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		recalls = append(recalls, domain.RecallResult{
			ProductID:  hit.Source.ID,
			Score:      hit.Score,
			Source:     domain.RecallKeyword,
			SalesCount: hit.Source.SalesCount,
		})
	}
	return recalls, nil
}

// ==================== 辅助类型与函数 ====================

type esSearchResult struct {
	Hits struct {
		Total struct{ Value int64 } `json:"total"`
		Hits  []esHit               `json:"hits"`
	} `json:"hits"`
}

type esHit struct {
	Source    domain.ProductDocument `json:"_source"`
	Score     float32                `json:"_score"`
	Highlight map[string][]string    `json:"highlight"`
}

type esAggResult struct {
	Aggs struct {
		CategoryAgg struct {
			Buckets []struct {
				Key   string `json:"key"`
				Count int64  `json:"doc_count"`
			} `json:"buckets"`
		} `json:"category_agg"`
		PriceRangeAgg struct {
			Buckets []struct {
				Key   string          `json:"key"`
				From  json.RawMessage `json:"from,omitempty"`
				To    json.RawMessage `json:"to,omitempty"`
				Count int64           `json:"doc_count"`
			} `json:"buckets"`
		} `json:"price_range_agg"`
	} `json:"aggregations"`
}

func hitToProduct(hit esHit, highlight bool) domain.ProductSearchResult {
	p := domain.ProductSearchResult{
		ID: hit.Source.ID, Name: hit.Source.Name,
		Description: hit.Source.Description, Picture: hit.Source.Picture,
		SliderImgs: hit.Source.SliderImgs, Price: hit.Source.Price,
		Categories: hit.Source.Categories, InStock: hit.Source.InStock,
		MerchantID: hit.Source.MerchantID, MerchantName: hit.Source.MerchantName,
		SalesCount: hit.Source.SalesCount, Score: hit.Score,
	}
	if highlight {
		if h, ok := hit.Highlight["name"]; ok && len(h) > 0 {
			p.NameHighlight = h[0]
		}
		if h, ok := hit.Highlight["description"]; ok && len(h) > 0 {
			p.DescriptionHighlight = h[0]
		}
	}
	return p
}

func parseESPrice(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0
	}
	return int64(f)
}

func formatPriceLabel(min, max int64) string {
	minY, maxY := float64(min)/100, float64(max)/100
	switch {
	case max == 0:
		return fmt.Sprintf("%.0f元以上", minY)
	case min == 0:
		return fmt.Sprintf("%.0f元以下", maxY)
	default:
		return fmt.Sprintf("%.0f-%.0f元", minY, maxY)
	}
}
