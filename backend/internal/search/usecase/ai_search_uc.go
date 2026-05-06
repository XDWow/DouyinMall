package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	es "github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const (
	recallTopK      = 100
	defaultPage     = int64(1)
	defaultPageSize = int64(10)
)

const queryUnderstandingPrompt = `You are an e-commerce query understanding assistant.
Return only JSON with this shape:
{
  "rewritten_query": "",
  "categories": [],
  "min_price": 0,
  "max_price": 0,
  "sort_by": "",
  "intent": "",
  "need_rag": true
}

Rules:
- rewritten_query: normalized search query
- categories: inferred product categories
- min_price/max_price: price range in cents, use 0 when absent
- sort_by: one of "", "PRICE_ASC", "PRICE_DESC", "SALES_DESC"
- intent: short natural-language summary of user intent
- need_rag: whether a natural-language product summary would help`

const ragPrompt = `You are an e-commerce shopping assistant.
Summarize the candidate products in concise Chinese.

Requirements:
1. Mention the approximate price range.
2. Highlight the main differences between products.
3. Mention stock or sales signals when useful.
4. Keep the answer to one or two short paragraphs.
5. Do not invent facts that are not in the candidate list.`

type AISearchUseCase struct {
	llmClient   ai.LLMClient
	embedder    ai.Embedder
	productRepo domain.ProductRepo
	l           logger.LoggerV1
}

func NewAISearchUseCase(
	llmClient ai.LLMClient,
	embedder ai.Embedder,
	productRepo domain.ProductRepo,
	l logger.LoggerV1,
) *AISearchUseCase {
	return &AISearchUseCase{
		llmClient:   llmClient,
		embedder:    embedder,
		productRepo: productRepo,
		l:           l,
	}
}

func (uc *AISearchUseCase) Execute(ctx context.Context, req *domain.AISearchProductsReq) (*domain.AISearchProductsResp, error) {
	if req == nil {
		return nil, fmt.Errorf("ai search request is nil")
	}

	page, pageSize := normalizePaging(req.Page, req.PageSize)
	pipelineStart := time.Now()
	metrics := &domain.PipelineMetrics{}

	t0 := time.Now()
	intent, err := uc.understandQuery(ctx, req.Query)
	if err != nil {
		uc.l.Warn("query understanding failed, fallback to original query", logger.Error(err))
		intent = defaultIntent(req.Query)
	}
	metrics.QueryUnderstandingMs = time.Since(t0).Milliseconds()

	t0 = time.Now()
	kwResults, vecResults, kwMs, vecMs := uc.multiRecall(ctx, intent)
	metrics.KeywordRecallMs = kwMs
	metrics.VectorRecallMs = vecMs
	metrics.KeywordRecallCount = int32(len(kwResults))
	metrics.VectorRecallCount = int32(len(vecResults))

	t0 = time.Now()
	candidates := uc.fusionRank(kwResults, vecResults, intent)
	rankedIDs := reRankBySales(candidates)
	metrics.RankingMs = time.Since(t0).Milliseconds()

	pageStart := (page - 1) * pageSize
	pageEnd := pageStart + pageSize
	if pageEnd > int64(len(rankedIDs)) {
		pageEnd = int64(len(rankedIDs))
	}
	if pageStart >= int64(len(rankedIDs)) {
		metrics.TotalMs = time.Since(pipelineStart).Milliseconds()
		return &domain.AISearchProductsResp{
			Products:    nil,
			Total:       int64(len(rankedIDs)),
			Page:        page,
			PageSize:    pageSize,
			QueryIntent: intent,
			Metrics:     metrics,
		}, nil
	}

	pageIDs := rankedIDs[pageStart:pageEnd]

	t0 = time.Now()
	products, err := uc.fetchProductsByIDs(ctx, pageIDs, req.EnableHighlight, intent.RewrittenQuery)
	if err != nil {
		return nil, fmt.Errorf("fetch products by ids: %w", err)
	}
	metrics.FetchMs = time.Since(t0).Milliseconds()

	resp := &domain.AISearchProductsResp{
		Products:    products,
		Total:       int64(len(rankedIDs)),
		Page:        page,
		PageSize:    pageSize,
		QueryIntent: intent,
		Metrics:     metrics,
	}

	if req.EnableRAG && intent.NeedRAG && len(products) > 0 {
		t0 = time.Now()
		summary, ragErr := uc.generateRAGSummary(ctx, req.Query, intent, products)
		metrics.RAGMs = time.Since(t0).Milliseconds()
		if ragErr != nil {
			uc.l.Warn("generate rag summary failed", logger.Error(ragErr))
		} else {
			resp.RAGSummary = summary
		}
	}

	metrics.TotalMs = time.Since(pipelineStart).Milliseconds()
	uc.l.Info("ai search completed",
		logger.Int64("total_ms", metrics.TotalMs),
		logger.Int64("query_understanding_ms", metrics.QueryUnderstandingMs),
		logger.Int64("keyword_recall_ms", metrics.KeywordRecallMs),
		logger.Int64("vector_recall_ms", metrics.VectorRecallMs),
		logger.Int64("ranking_ms", metrics.RankingMs),
		logger.Int64("fetch_ms", metrics.FetchMs),
		logger.Int64("rag_ms", metrics.RAGMs),
	)

	return resp, nil
}

func (uc *AISearchUseCase) understandQuery(ctx context.Context, query string) (*domain.QueryIntent, error) {
	resp, err := uc.llmClient.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: queryUnderstandingPrompt},
			{Role: "user", Content: query},
		},
		Temperature: float32Ptr(0.1),
		MaxTokens:   intPtr(256),
	})
	if err != nil {
		return nil, err
	}

	content := cleanModelContent(chatContent(resp))
	if content == "" {
		return defaultIntent(query), nil
	}

	var parsed struct {
		RewrittenQuery string   `json:"rewritten_query"`
		Categories     []string `json:"categories"`
		MinPrice       int64    `json:"min_price"`
		MaxPrice       int64    `json:"max_price"`
		SortBy         string   `json:"sort_by"`
		Intent         string   `json:"intent"`
		NeedRAG        *bool    `json:"need_rag"`
	}

	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		uc.l.Warn("parse query intent failed", logger.Error(err), logger.String("raw", content))
		return defaultIntent(query), nil
	}

	intent := defaultIntent(query)
	if strings.TrimSpace(parsed.RewrittenQuery) != "" {
		intent.RewrittenQuery = strings.TrimSpace(parsed.RewrittenQuery)
	}
	intent.Categories = parsed.Categories
	intent.MinPrice = parsed.MinPrice
	intent.MaxPrice = parsed.MaxPrice
	intent.SortBy = strings.TrimSpace(parsed.SortBy)
	intent.Intent = strings.TrimSpace(parsed.Intent)
	if parsed.NeedRAG != nil {
		intent.NeedRAG = *parsed.NeedRAG
	}

	return intent, nil
}

func (uc *AISearchUseCase) multiRecall(ctx context.Context, intent *domain.QueryIntent) (
	keyword, vector []domain.RecallResult, kwMs, vecMs int64,
) {
	filters := es.BuildFiltersFromIntent(intent)

	t0 := time.Now()
	kwQuery := es.BuildKeywordQuery(intent.RewrittenQuery, recallTopK, filters)
	var kwErr error
	keyword, kwErr = uc.productRepo.KeywordRecallSearch(ctx, kwQuery)
	kwMs = time.Since(t0).Milliseconds()
	if kwErr != nil {
		uc.l.Warn("keyword recall failed", logger.Error(kwErr))
		keyword = nil
	}

	t0 = time.Now()
	vectors, embErr := uc.embedder.Embed(ctx, []string{intent.RewrittenQuery})
	if embErr != nil {
		uc.l.Warn("embedding failed, skip vector recall", logger.Error(embErr))
		vecMs = time.Since(t0).Milliseconds()
		return keyword, nil, kwMs, vecMs
	}

	if len(vectors) > 0 {
		var vecErr error
		vector, vecErr = uc.productRepo.VectorSearch(ctx, vectors[0], recallTopK, filters)
		if vecErr != nil {
			uc.l.Warn("vector recall failed", logger.Error(vecErr))
			vector = nil
		}
	}
	vecMs = time.Since(t0).Milliseconds()
	return
}

type rankCandidate struct {
	id         int64
	rrfScore   float64
	salesCount int64
}

func (uc *AISearchUseCase) fusionRank(keyword, vector []domain.RecallResult, intent *domain.QueryIntent) []rankCandidate {
	const rrfK = 60

	kwWeight, vecWeight := 1.0, 1.0
	if intent != nil && intent.NeedRAG {
		kwWeight, vecWeight = 0.8, 1.2
	} else {
		kwWeight, vecWeight = 1.2, 0.8
	}

	type docSignal struct {
		rrfScore   float64
		salesCount int64
	}

	docs := make(map[int64]*docSignal, len(keyword)+len(vector))
	upsert := func(id int64, salesCount int64) *docSignal {
		if d, ok := docs[id]; ok {
			if salesCount > d.salesCount {
				d.salesCount = salesCount
			}
			return d
		}
		d := &docSignal{salesCount: salesCount}
		docs[id] = d
		return d
	}

	for rank, result := range keyword {
		upsert(result.ProductID, result.SalesCount).rrfScore += kwWeight / float64(rrfK+rank+1)
	}
	for rank, result := range vector {
		upsert(result.ProductID, result.SalesCount).rrfScore += vecWeight / float64(rrfK+rank+1)
	}

	candidates := make([]rankCandidate, 0, len(docs))
	for id, signal := range docs {
		candidates = append(candidates, rankCandidate{
			id:         id,
			rrfScore:   signal.rrfScore,
			salesCount: signal.salesCount,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rrfScore == candidates[j].rrfScore {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].rrfScore > candidates[j].rrfScore
	})

	return candidates
}

func reRankBySales(candidates []rankCandidate) []int64 {
	type idScore struct {
		id    int64
		score float64
	}

	ranked := make([]idScore, len(candidates))
	for i, candidate := range candidates {
		salesBoost := 1.0 + 0.1*math.Log2(1.0+float64(candidate.salesCount))
		ranked[i] = idScore{
			id:    candidate.id,
			score: candidate.rrfScore * salesBoost,
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].id < ranked[j].id
		}
		return ranked[i].score > ranked[j].score
	})

	ids := make([]int64, len(ranked))
	for i, result := range ranked {
		ids[i] = result.id
	}
	return ids
}

func (uc *AISearchUseCase) fetchProductsByIDs(ctx context.Context, ids []int64, highlight bool, keyword string) ([]domain.ProductSearchResult, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	products, err := uc.productRepo.GetProductsByIDs(ctx, ids, highlight, keyword)
	if err != nil {
		return nil, err
	}

	productByID := make(map[int64]domain.ProductSearchResult, len(products))
	for _, product := range products {
		productByID[product.ID] = product
	}

	ordered := make([]domain.ProductSearchResult, 0, len(products))
	for _, id := range ids {
		product, ok := productByID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, product)
	}

	return ordered, nil
}

func (uc *AISearchUseCase) generateRAGSummary(
	ctx context.Context,
	query string,
	intent *domain.QueryIntent,
	products []domain.ProductSearchResult,
) (string, error) {
	topN := 8
	if len(products) < topN {
		topN = len(products)
	}

	var sb strings.Builder
	var minPrice, maxPrice int64
	categorySet := make(map[string]int)
	var bestSeller string
	var bestSales int64

	for i := 0; i < topN; i++ {
		product := products[i]
		priceYuan := float64(product.Price) / 100
		fmt.Fprintf(&sb, "%d. %s | %.2f yuan | sales %d | %s\n",
			i+1, product.Name, priceYuan, product.SalesCount, strings.Join(product.Categories, "/"))

		if minPrice == 0 || product.Price < minPrice {
			minPrice = product.Price
		}
		if product.Price > maxPrice {
			maxPrice = product.Price
		}
		for _, category := range product.Categories {
			categorySet[category]++
		}
		if product.SalesCount > bestSales {
			bestSales = product.SalesCount
			bestSeller = product.Name
		}
	}

	fmt.Fprintf(&sb, "\nPrice range: %.2f-%.2f yuan.", float64(minPrice)/100, float64(maxPrice)/100)
	if bestSeller != "" {
		fmt.Fprintf(&sb, " Best seller: %s (sales %d).", bestSeller, bestSales)
	}
	if len(categorySet) > 0 {
		categories := make([]string, 0, len(categorySet))
		for category := range categorySet {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		fmt.Fprintf(&sb, " Categories: %s.", strings.Join(categories, ", "))
	}

	resp, err := uc.llmClient.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: ragPrompt},
			{
				Role: "user",
				Content: fmt.Sprintf("User query: %s\nIntent: %s\n\nCandidate products:\n%s",
					query, intent.Intent, sb.String()),
			},
		},
		Temperature: float32Ptr(0.3),
		MaxTokens:   intPtr(300),
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(chatContent(resp)), nil
}

func normalizePaging(page, pageSize int64) (int64, int64) {
	if page <= 0 {
		page = defaultPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

func defaultIntent(query string) *domain.QueryIntent {
	return &domain.QueryIntent{
		RewrittenQuery: strings.TrimSpace(query),
		NeedRAG:        true,
	}
}

func chatContent(resp *ai.ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

func cleanModelContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func float32Ptr(v float32) *float32 {
	return &v
}

func intPtr(v int) *int {
	return &v
}
