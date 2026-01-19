package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo/es"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type productRepo struct {
	esClient *es.ESClient
	logger   logger.LoggerV1
}

func NewProductRepo(esClient *es.ESClient, logger logger.LoggerV1) ProductRepo {
	return &productRepo{
		esClient: esClient,
		logger:   logger,
	}
}

func (r *productRepo) SearchProducts(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchProductsResp, error) {
	start := time.Now()
	query := r.buildProductQuery(req)
	res, err := r.esClient.Search(ctx, es.ProductIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source    domain.ProductDocument `json:"_source"`
				Score     float32                `json:"_score"`
				Highlight map[string][]string    `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	products := make([]domain.ProductSearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		product := domain.ProductSearchResult{
			ID:           hit.Source.ID,
			Name:         hit.Source.Name,
			Description:  hit.Source.Description,
			Picture:      hit.Source.Picture,
			SliderImgs:   hit.Source.SliderImgs,
			Price:        hit.Source.Price,
			Categories:   hit.Source.Categories,
			InStock:      hit.Source.InStock,
			MerchantID:   hit.Source.MerchantID,
			MerchantName: hit.Source.MerchantName,
			Score:        hit.Score,
		}

		// 处理高亮
		if req.EnableHighlight {
			if highlights, ok := hit.Highlight["name"]; ok && len(highlights) > 0 {
				product.NameHighlight = highlights[0]
			}
			if highlights, ok := hit.Highlight["description"]; ok && len(highlights) > 0 {
				product.DescriptionHighlight = highlights[0]
			}
		}

		products = append(products, product)
	}

	tookMs := time.Since(start).Milliseconds()
	r.logger.Info("商品搜索完成",
		logger.Int64("took_ms", tookMs),
		logger.Int64("total", result.Hits.Total.Value))

	return &domain.SearchProductsResp{
		Products: products,
		Total:    result.Hits.Total.Value,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (r *productRepo) buildProductQuery(req *domain.SearchProductsReq) string {
	var query strings.Builder

	query.WriteString(`{
		"from": `)
	query.WriteString(strconv.FormatInt((req.Page-1)*req.PageSize, 10))
	query.WriteString(`,
		"size": `)
	query.WriteString(strconv.FormatInt(req.PageSize, 10))
	query.WriteString(`,
		"query": {
			"bool": {
				"must": [],
				"filter": []
			}
		},
		"sort": []
	}`)

	var queryObj map[string]interface{}
	json.Unmarshal([]byte(query.String()), &queryObj)

	boolQuery := queryObj["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := boolQuery["must"].([]interface{})
	filter := boolQuery["filter"].([]interface{})

	// 关键字搜索
	if req.Keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  req.Keyword,
				"fields": []string{"name^3", "description"},
			},
		})
	}

	// 分类筛选
	if len(req.Categories) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{
				"categories": req.Categories,
			},
		})
	}

	// 价格区间
	if req.MinPrice > 0 || req.MaxPrice > 0 {
		priceRange := map[string]interface{}{}
		if req.MinPrice > 0 {
			priceRange["gte"] = req.MinPrice
		}
		if req.MaxPrice > 0 {
			priceRange["lte"] = req.MaxPrice
		}
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"price": priceRange,
			},
		})
	}

	// 商家筛选
	if req.MerchantID > 0 {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"merchant_id": req.MerchantID,
			},
		})
	}

	// 库存筛选（只看有货）
	if req.InStockOnly {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"in_stock": true,
			},
		})
	}

	// 排序
	sort := queryObj["sort"].([]interface{})
	switch req.SortBy {
	case "PRICE_ASC":
		sort = append(sort, map[string]interface{}{"price": "asc"})
	case "PRICE_DESC":
		sort = append(sort, map[string]interface{}{"price": "desc"})
	case "SALES_DESC":
		// 预留：销量排序（需要添加 sales_count 字段）
		sort = append(sort, map[string]interface{}{"_score": "desc"})
	case "NEW_ARRIVAL":
		sort = append(sort, map[string]interface{}{"created_at": "desc"})
	default: // RELEVANCE
		sort = append(sort, map[string]interface{}{"_score": "desc"})
	}

	// 高亮
	if req.EnableHighlight {
		queryObj["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"name":        map[string]interface{}{},
				"description": map[string]interface{}{},
			},
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
		}
	}

	boolQuery["must"] = must
	boolQuery["filter"] = filter
	queryObj["sort"] = sort

	queryJSON, _ := json.Marshal(queryObj)
	return string(queryJSON)
}

func (r *productRepo) SearchProductSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	// 商品建议需要查询数量
	suggestions, err := r.esClient.SearchSuggest(ctx, es.ProductIndex, keyword, limit, true)
	if err != nil {
		return nil, err
	}

	result := make([]domain.SearchSuggestion, 0, len(suggestions))
	for _, sug := range suggestions {
		if text, ok := sug["text"].(string); ok {
			s := domain.SearchSuggestion{
				Keyword: text,
				Source:  "NAME_MATCH",
			}
			if score, ok := sug["score"].(float64); ok {
				s.Count = int64(score)
			}
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *productRepo) GetAggregations(ctx context.Context, req *domain.SearchProductsReq) (*domain.SearchAggregationsResp, error) {
	// 构建聚合查询（复用搜索查询的过滤条件，但不返回文档，只返回聚合结果）
	query := r.buildAggregationQuery(req)

	res, err := r.esClient.Search(ctx, es.ProductIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// 定义价格区间桶结构体
	// 注意：ES 的 range aggregation 返回的 JSON 中 from/to 是 float64 格式，
	// 但字段在 ES 中是 long (int64) 类型，所以需要解析时转换为 int64
	type priceRangeBucket struct {
		Key          string          `json:"key"`
		From         json.RawMessage `json:"from,omitempty"` // 使用 RawMessage 先接收原始值
		To           json.RawMessage `json:"to,omitempty"`   // 使用 RawMessage 先接收原始值
		Count        int64           `json:"doc_count"`
		FromAsString string          `json:"from_as_string,omitempty"`
		ToAsString   string          `json:"to_as_string,omitempty"`
	}

	// 解析数值为 int64 的辅助函数（处理 ES 返回的 float64 格式）
	parseInt64FromES := func(raw json.RawMessage) (int64, error) {
		if len(raw) == 0 {
			return 0, nil
		}
		// ES 可能返回 float64 格式（如 5000.0），先解析为 float64 再转换为 int64
		var f float64
		if err := json.Unmarshal(raw, &f); err != nil {
			// 如果解析失败，尝试直接解析为 int64
			var i int64
			if err2 := json.Unmarshal(raw, &i); err2 != nil {
				return 0, fmt.Errorf("无法解析价格值: %w", err)
			}
			return i, nil
		}
		return int64(f), nil
	}

	var result struct {
		Aggregations struct {
			CategoryAgg struct {
				Buckets []struct {
					Key   string `json:"key"`
					Count int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"category_agg"`
			PriceRangeAgg struct {
				Buckets []priceRangeBucket `json:"buckets"`
			} `json:"price_range_agg"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析聚合结果失败: %w", err)
	}

	// 解析分类聚合
	categories := make([]domain.CategoryAggregation, 0, len(result.Aggregations.CategoryAgg.Buckets))
	for _, bucket := range result.Aggregations.CategoryAgg.Buckets {
		categories = append(categories, domain.CategoryAggregation{
			Category: bucket.Key,
			Count:    bucket.Count,
		})
	}

	// 解析价格区间聚合（ES 存储是 long/int64，但返回 JSON 中可能是 float64，需要转换为 int64）
	priceRanges := make([]domain.PriceRangeAggregation, 0, len(result.Aggregations.PriceRangeAgg.Buckets))
	for _, bucket := range result.Aggregations.PriceRangeAgg.Buckets {
		// 从 ES 返回的原始值解析为 int64（ES 存储是 long，但 JSON 返回可能是 float64）
		fromPrice, err := parseInt64FromES(bucket.From)
		if err != nil {
			return nil, fmt.Errorf("解析价格区间 from 失败: %w", err)
		}
		toPrice, err := parseInt64FromES(bucket.To)
		if err != nil {
			return nil, fmt.Errorf("解析价格区间 to 失败: %w", err)
		}
		label := r.formatPriceRangeLabel(fromPrice, toPrice)
		priceRanges = append(priceRanges, domain.PriceRangeAggregation{
			MinPrice: fromPrice, // int64，避免精度问题
			MaxPrice: toPrice,   // int64，避免精度问题
			Count:    bucket.Count,
			Label:    label,
		})
	}

	return &domain.SearchAggregationsResp{
		Categories:  categories,
		PriceRanges: priceRanges,
	}, nil
}

func (r *productRepo) buildAggregationQuery(req *domain.SearchProductsReq) string {
	// 先构建基础查询（复用）
	baseQuery := r.buildProductQuery(req)

	var queryObj map[string]interface{}
	if err := json.Unmarshal([]byte(baseQuery), &queryObj); err != nil {
		return baseQuery
	}

	// 不需要返回文档，只需要聚合结果
	queryObj["size"] = 0

	// 添加聚合配置
	queryObj["aggs"] = map[string]interface{}{
		// 分类聚合
		"category_agg": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "categories",
				"size":  100, // 最多返回 100 个分类
			},
		},
		// 价格区间聚合：统计不同价格区间的商品数量
		"price_range_agg": map[string]interface{}{
			"range": map[string]interface{}{
				"field": "price",
				"ranges": []map[string]interface{}{
					{"key": "0-50", "to": 5000},                      // 0-50元（单位：分）
					{"key": "50-100", "from": 5000, "to": 10000},     // 50-100元
					{"key": "100-200", "from": 10000, "to": 20000},   // 100-200元
					{"key": "200-500", "from": 20000, "to": 50000},   // 200-500元
					{"key": "500-1000", "from": 50000, "to": 100000}, // 500-1000元
					{"key": "1000+", "from": 100000},                 // 1000元以上
				},
			},
		},
	}

	queryJSON, _ := json.Marshal(queryObj)
	return string(queryJSON)
}

// 格式化价格区间标签
func (r *productRepo) formatPriceRangeLabel(min, max int64) string {
	minYuan := float64(min) / 100
	maxYuan := float64(max) / 100

	if max == 0 {
		return fmt.Sprintf("%.0f元以上", minYuan)
	}

	if min == 0 {
		return fmt.Sprintf("%.0f元以下", maxYuan)
	}

	return fmt.Sprintf("%.0f-%.0f元", minYuan, maxYuan)
}

func (r *productRepo) SyncProduct(ctx context.Context, action string, doc *domain.ProductDocument) error {
	docID := strconv.FormatInt(doc.ID, 10)

	switch action {
	case "CREATE":
		return r.esClient.IndexDocument(ctx, es.ProductIndex, docID, doc)
	case "UPDATE":
		return r.esClient.UpdateDocument(ctx, es.ProductIndex, docID, doc)
	case "DELETE":
		return r.esClient.DeleteDocument(ctx, es.ProductIndex, docID)
	default:
		return fmt.Errorf("不支持的操作类型: %s", action)
	}
}

func (r *productRepo) BatchSyncProducts(ctx context.Context, docs []domain.ProductDocument) (successCount, failedCount int64, errors []string) {
	// 转换为 map 格式
	docMaps := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		docMap := map[string]interface{}{
			"id":            doc.ID,
			"name":          doc.Name,
			"description":   doc.Description,
			"picture":       doc.Picture,
			"slider_imgs":   doc.SliderImgs,
			"price":         doc.Price,
			"categories":    doc.Categories,
			"in_stock":      doc.InStock,
			"merchant_id":   doc.MerchantID,
			"merchant_name": doc.MerchantName,
			"created_at":    doc.CreatedTime,
			"updated_at":    doc.UpdatedTime,
		}
		docMaps = append(docMaps, docMap)
	}

	if err := r.esClient.BulkIndex(ctx, es.ProductIndex, docMaps); err != nil {
		return 0, int64(len(docs)), []string{err.Error()}
	}

	return int64(len(docs)), 0, nil
}

func (r *productRepo) DeleteProduct(ctx context.Context, productID int64) error {
	docID := strconv.FormatInt(productID, 10)
	return r.esClient.DeleteDocument(ctx, es.ProductIndex, docID)
}

func (r *productRepo) BatchDeleteProducts(ctx context.Context, productIDs []int64) (successCount, failedCount int64, errors []string) {
	if len(productIDs) == 0 {
		return 0, 0, nil
	}

	docIDs := make([]string, len(productIDs))
	for i, id := range productIDs {
		docIDs[i] = strconv.FormatInt(id, 10)
	}

	if err := r.esClient.BulkDelete(ctx, es.ProductIndex, docIDs); err != nil {
		return 0, int64(len(productIDs)), []string{err.Error()}
	}

	return int64(len(productIDs)), 0, nil
}
