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

type merchantRepo struct {
	esClient *es.ESClient
	logger   logger.LoggerV1
}

func NewMerchantRepo(esClient *es.ESClient, logger logger.LoggerV1) MerchantRepo {
	return &merchantRepo{
		esClient: esClient,
		logger:   logger,
	}
}

func (r *merchantRepo) SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error) {
	start := time.Now()
	query := r.buildMerchantQuery(req)
	res, err := r.esClient.Search(ctx, es.MerchantIndex, query)
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
				Source    domain.MerchantDocument `json:"_source"`
				Score     float32                 `json:"_score"`
				Highlight map[string][]string     `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	merchants := make([]domain.MerchantSearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		merchant := domain.MerchantSearchResult{
			ID:           hit.Source.ID,
			Name:         hit.Source.Name,
			Description:  hit.Source.Description,
			Logo:         hit.Source.Logo,
			Region:       hit.Source.Region,
			Rating:       hit.Source.Rating,
			SalesCount:   hit.Source.SalesCount,
			ProductCount: hit.Source.ProductCount,
			Verified:     hit.Source.Verified,
			Score:        hit.Score,
		}

		// 处理高亮
		if highlights, ok := hit.Highlight["name"]; ok && len(highlights) > 0 {
			merchant.NameHighlight = highlights[0]
		}

		merchants = append(merchants, merchant)
	}

	tookMs := time.Since(start).Milliseconds()
	r.logger.Info("商家搜索完成",
		logger.Int64("took_ms", tookMs),
		logger.Int64("total", result.Hits.Total.Value))

	return &domain.SearchMerchantsResp{
		Merchants: merchants,
		Total:     result.Hits.Total.Value,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}, nil
}

func (r *merchantRepo) buildMerchantQuery(req *domain.SearchMerchantsReq) string {
	var query strings.Builder
	query.WriteString(`{
		"query": {
			"bool": {
				"must": [],
				"filter": []
			}
		},
		"sort": [],
		"from": `)
	query.WriteString(strconv.FormatInt((req.Page-1)*req.PageSize, 10))
	query.WriteString(`,
		"size": `)
	query.WriteString(strconv.FormatInt(req.PageSize, 10))
	query.WriteString(`
	}`)

	var queryObj map[string]interface{}
	if err := json.Unmarshal([]byte(query.String()), &queryObj); err != nil {
		return query.String()
	}

	boolQuery := queryObj["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := boolQuery["must"].([]interface{})
	filter := boolQuery["filter"].([]interface{})

	// 关键词搜索
	if req.Keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  req.Keyword,
				"fields": []string{"name^3", "description"},
			},
		})
	}

	// 地区筛选
	if req.Region != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"region": req.Region,
			},
		})
	}

	// 最低评分筛选
	if req.MinRating > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"rating": map[string]interface{}{
					"gte": req.MinRating,
				},
			},
		})
	}

	// 认证商家筛选
	if req.Verified != nil && *req.Verified {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"verified": true,
			},
		})
	}

	boolQuery["must"] = must
	boolQuery["filter"] = filter

	// 排序
	sort := queryObj["sort"].([]interface{})
	switch req.SortBy {
	case "SALES_DESC":
		sort = append(sort, map[string]interface{}{
			"sales_count": map[string]interface{}{
				"order": "desc",
			},
		})
	case "RATING_DESC":
		sort = append(sort, map[string]interface{}{
			"rating": map[string]interface{}{
				"order": "desc",
			},
		})
	case "NEW_ARRIVAL":
		sort = append(sort, map[string]interface{}{
			"created_at": map[string]interface{}{
				"order": "desc",
			},
		})
	default: // RELEVANCE 或默认
		sort = append(sort, "_score")
	}
	queryObj["sort"] = sort

	queryJSON, _ := json.Marshal(queryObj)
	return string(queryJSON)
}

func (r *merchantRepo) SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	// 商家建议不需要查询数量
	suggestions, err := r.esClient.SearchSuggest(ctx, es.MerchantIndex, keyword, limit, false)
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
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *merchantRepo) SyncMerchant(ctx context.Context, action string, doc *domain.MerchantDocument) error {
	docID := strconv.FormatInt(doc.ID, 10)

	switch action {
	case "CREATE":
		return r.esClient.IndexDocument(ctx, es.MerchantIndex, docID, doc)
	case "UPDATE":
		return r.esClient.UpdateDocument(ctx, es.MerchantIndex, docID, doc)
	case "DELETE":
		return r.esClient.DeleteDocument(ctx, es.MerchantIndex, docID)
	default:
		return fmt.Errorf("不支持的操作类型: %s", action)
	}
}

func (r *merchantRepo) BatchSyncMerchants(ctx context.Context, docs []domain.MerchantDocument) (successCount, failedCount int64, errors []string) {
	// 转换为 map 格式
	docMaps := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		docMap := map[string]interface{}{
			"id":             doc.ID,
			"name":           doc.Name,
			"description":    doc.Description,
			"logo":           doc.Logo,
			"region":         doc.Region,
			"rating":         doc.Rating,
			"sales_count":    doc.SalesCount,
			"product_count":  doc.ProductCount,
			"verified":       doc.Verified,
			"created_at":     doc.CreatedTime,
			"updated_at":     doc.UpdatedTime,
		}
		docMaps = append(docMaps, docMap)
	}

	if err := r.esClient.BulkIndex(ctx, es.MerchantIndex, docMaps); err != nil {
		return 0, int64(len(docs)), []string{err.Error()}
	}

	return int64(len(docs)), 0, nil
}

func (r *merchantRepo) DeleteMerchant(ctx context.Context, merchantID int64) error {
	docID := strconv.FormatInt(merchantID, 10)
	return r.esClient.DeleteDocument(ctx, es.MerchantIndex, docID)
}

func (r *merchantRepo) BatchDeleteMerchants(ctx context.Context, merchantIDs []int64) (successCount, failedCount int64, errors []string) {
	if len(merchantIDs) == 0 {
		return 0, 0, nil
	}

	docIDs := make([]string, len(merchantIDs))
	for i, id := range merchantIDs {
		docIDs[i] = strconv.FormatInt(id, 10)
	}

	if err := r.esClient.BulkDelete(ctx, es.MerchantIndex, docIDs); err != nil {
		return 0, int64(len(merchantIDs)), []string{err.Error()}
	}

	return int64(len(merchantIDs)), 0, nil
}

