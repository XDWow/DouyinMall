package es

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type merchantRepo struct {
	es *ESClient
	l  logger.LoggerV1
}

// NewMerchantRepo 鍒涘缓鍟嗗浠撳偍锛堝疄鐜?domain.MerchantRepo 绔彛锛?
func NewMerchantRepo(esClient *ESClient, l logger.LoggerV1) domain.MerchantRepo {
	return &merchantRepo{es: esClient, l: l}
}

func (r *merchantRepo) SearchMerchants(ctx context.Context, req *domain.SearchMerchantsReq) (*domain.SearchMerchantsResp, error) {
	query := r.buildMerchantQuery(req)
	res, err := r.es.Search(ctx, MerchantIndex, query)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total struct{ Value int64 } `json:"total"`
			Hits  []struct {
				Source    domain.MerchantDocument `json:"_source"`
				Score     float32                 `json:"_score"`
				Highlight map[string][]string     `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("瑙ｆ瀽鍟嗗鎼滅储缁撴灉澶辫触: %w", err)
	}

	merchants := make([]domain.MerchantSearchResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		m := domain.MerchantSearchResult{
			ID: hit.Source.ID, Name: hit.Source.Name,
			Description: hit.Source.Description, Logo: hit.Source.Logo,
			Region: hit.Source.Region, Rating: hit.Source.Rating,
			SalesCount: hit.Source.SalesCount, ProductCount: hit.Source.ProductCount,
			Verified: hit.Source.Verified, Score: hit.Score,
		}
		if h, ok := hit.Highlight["name"]; ok && len(h) > 0 {
			m.NameHighlight = h[0]
		}
		merchants = append(merchants, m)
	}

	return &domain.SearchMerchantsResp{
		Merchants: merchants, Total: result.Hits.Total.Value,
		Page: req.Page, PageSize: req.PageSize,
	}, nil
}

func (r *merchantRepo) SearchMerchantSuggest(ctx context.Context, keyword string, limit int64) ([]domain.SearchSuggestion, error) {
	suggestions, err := r.es.SearchSuggest(ctx, MerchantIndex, keyword, limit)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SearchSuggestion, 0, len(suggestions))
	for _, sug := range suggestions {
		if text, ok := sug["text"].(string); ok {
			result = append(result, domain.SearchSuggestion{Keyword: text, Source: "NAME_MATCH"})
		}
	}
	return result, nil
}

// ==================== 绱㈠紩绠＄悊 ====================

func (r *merchantRepo) SyncMerchant(ctx context.Context, action string, doc *domain.MerchantDocument) error {
	docID := strconv.FormatInt(doc.ID, 10)
	switch action {
	case "CREATE":
		return r.es.IndexDocument(ctx, MerchantIndex, docID, doc)
	case "UPDATE":
		return r.es.UpdateDocument(ctx, MerchantIndex, docID, doc)
	case "DELETE":
		return r.es.DeleteDocument(ctx, MerchantIndex, docID)
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑鎿嶄綔: %s", action)
	}
}

func (r *merchantRepo) BatchSyncMerchants(ctx context.Context, docs []domain.MerchantDocument) (int64, int64, []string) {
	docMaps := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		docMaps = append(docMaps, map[string]interface{}{
			"id": doc.ID, "name": doc.Name, "description": doc.Description,
			"logo": doc.Logo, "region": doc.Region, "rating": doc.Rating,
			"sales_count": doc.SalesCount, "product_count": doc.ProductCount,
			"verified":   doc.Verified,
			"created_at": doc.CreatedTime, "updated_at": doc.UpdatedTime,
		})
	}
	if err := r.es.BulkIndex(ctx, MerchantIndex, docMaps); err != nil {
		return 0, int64(len(docs)), []string{err.Error()}
	}
	return int64(len(docs)), 0, nil
}

func (r *merchantRepo) DeleteMerchant(ctx context.Context, merchantID int64) error {
	return r.es.DeleteDocument(ctx, MerchantIndex, strconv.FormatInt(merchantID, 10))
}

func (r *merchantRepo) BatchDeleteMerchants(ctx context.Context, ids []int64) (int64, int64, []string) {
	if len(ids) == 0 {
		return 0, 0, nil
	}
	docIDs := make([]string, len(ids))
	for i, id := range ids {
		docIDs[i] = strconv.FormatInt(id, 10)
	}
	if err := r.es.BulkDelete(ctx, MerchantIndex, docIDs); err != nil {
		return 0, int64(len(ids)), []string{err.Error()}
	}
	return int64(len(ids)), 0, nil
}

// ==================== 鏌ヨ鏋勫缓 ====================

func (r *merchantRepo) buildMerchantQuery(req *domain.SearchMerchantsReq) string {
	queryObj := map[string]interface{}{
		"from": (req.Page - 1) * req.PageSize,
		"size": req.PageSize,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": []interface{}{}, "filter": []interface{}{}},
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
	if req.Region != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"region": req.Region}})
	}
	if req.MinRating > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"rating": map[string]interface{}{"gte": req.MinRating}},
		})
	}
	if req.Verified != nil && *req.Verified {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"verified": true}})
	}

	switch req.SortBy {
	case "MERCHANT_SALES_DESC":
		sort = append(sort, map[string]interface{}{"sales_count": "desc"})
	case "MERCHANT_RATING_DESC":
		sort = append(sort, map[string]interface{}{"rating": "desc"})
	case "MERCHANT_NEW_ARRIVAL":
		sort = append(sort, map[string]interface{}{"created_at": "desc"})
	default:
		sort = append(sort, map[string]interface{}{"_score": "desc"})
	}

	queryObj["highlight"] = map[string]interface{}{
		"fields":    map[string]interface{}{"name": map[string]interface{}{}},
		"pre_tags":  []string{"<em>"},
		"post_tags": []string{"</em>"},
	}

	boolQ["must"] = must
	boolQ["filter"] = filter
	queryObj["sort"] = sort
	body, _ := json.Marshal(queryObj)
	return string(body)
}


