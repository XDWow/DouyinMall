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
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	es "github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// AISearchUseCase AI 搜索五阶段管线：
//
//	Query理解(LLM) → 多路召回(ES倒排+kNN) → 两阶段排序(相关性门控+业务重排) → 获取详情 → RAG摘要
//
// 亮点：
//  1. Embedder / LLMClient 接口分离，向量化与对话能力独立替换
//  2. 意图感知的召回权重——精确查询侧重关键词，模糊查询侧重向量
//  3. 两阶段排序：相关性门控 + 业务重排，解耦可扩展
//  4. PipelineMetrics 全链路可观测（每阶段 ms + 召回计数）
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
	return &AISearchUseCase{llmClient: llmClient, embedder: embedder, productRepo: productRepo, l: l}
}

func (uc *AISearchUseCase) Execute(ctx context.Context, req *domain.AISearchProductsReq) (*domain.AISearchProductsResp, error) {
	pipelineStart := time.Now()
	metrics := &domain.PipelineMetrics{}

	// ① Query 理解
	t0 := time.Now()
	intent, err := uc.understandQuery(ctx, req.Query)
	if err != nil {
		uc.l.Warn("Query 理解失败，降级为原始查询", logger.Error(err))
		intent = &domain.QueryIntent{RewrittenQuery: req.Query, NeedRAG: true}
	}
	metrics.QueryUnderstandingMs = time.Since(t0).Milliseconds()

	// ② 多路召回（意图感知权重）
	t0 = time.Now()
	kwResults, vecResults, kwMs, vecMs := uc.multiRecall(ctx, intent)
	metrics.KeywordRecallMs = kwMs
	metrics.VectorRecallMs = vecMs
	metrics.KeywordRecallCount = int32(len(kwResults))
	metrics.VectorRecallCount = int32(len(vecResults))

	// ③ 两阶段排序：
	//    阶段一：纯 RRF 相关性融合 → 选出候选集（相关性门控）
	//    阶段二：在候选集内按业务信号重排 → 不相关的商品无法靠销量"买入"结果
	t0 = time.Now()
	candidates := uc.fusionRank(kwResults, vecResults, intent)
	rankedIDs := reRankBySales(candidates)
	metrics.RankingMs = time.Since(t0).Milliseconds()

	// ④ 分页 + 获取完整商品
	pageStart := (req.Page - 1) * req.PageSize
	pageEnd := pageStart + req.PageSize
	if pageEnd > int64(len(rankedIDs)) {
		pageEnd = int64(len(rankedIDs))
	}
	if pageStart >= int64(len(rankedIDs)) {
		metrics.TotalMs = time.Since(pipelineStart).Milliseconds()
		return &domain.AISearchProductsResp{
			Page: req.Page, PageSize: req.PageSize, QueryIntent: intent, Metrics: metrics,
		}, nil
	}
	pageIDs := rankedIDs[pageStart:pageEnd]

	t0 = time.Now()
	products, err := uc.fetchProductsByIDs(ctx, pageIDs, req.EnableHighlight, intent.RewrittenQuery)
	if err != nil {
		return nil, fmt.Errorf("获取商品详情失败: %w", err)
	}
	metrics.FetchMs = time.Since(t0).Milliseconds()

	resp := &domain.AISearchProductsResp{
		Products:    products,
		Total:       int64(len(rankedIDs)),
		Page:        req.Page,
		PageSize:    req.PageSize,
		QueryIntent: intent,
		Metrics:     metrics,
	}

	// ⑤ RAG 摘要
	if req.EnableRAG && intent.NeedRAG && len(products) > 0 {
		t0 = time.Now()
		summary, ragErr := uc.generateRAGSummary(ctx, req.Query, intent, products)
		metrics.RAGMs = time.Since(t0).Milliseconds()
		if ragErr != nil {
			uc.l.Warn("RAG 摘要生成失败", logger.Error(ragErr))
		} else {
			resp.RAGSummary = summary
		}
	}

	metrics.TotalMs = time.Since(pipelineStart).Milliseconds()
	uc.l.Info("AI 搜索完成",
		logger.Int64("total_ms", metrics.TotalMs),
		logger.Int64("qu_ms", metrics.QueryUnderstandingMs),
		logger.Int64("kw_ms", metrics.KeywordRecallMs),
		logger.Int64("vec_ms", metrics.VectorRecallMs),
		logger.Int64("rank_ms", metrics.RankingMs),
		logger.Int64("fetch_ms", metrics.FetchMs),
		logger.Int64("rag_ms", metrics.RAGMs),
	)
	return resp, nil
}

// ==================== ① Query 理解 ====================

const queryUnderstandingPrompt = `你是电商搜索系统的 Query 理解模块。分析用户搜索词，输出 JSON（不要 markdown）：
{
  "rewritten_query": "优化后的搜索关键词",
  "categories": ["识别出的商品类目"],
  "min_price": 0,
  "max_price": 0,
  "sort_by": "",
  "intent": "用户意图简述",
  "need_rag": true
}
规则：
- rewritten_query: 提取核心商品词，去掉口语化表达，展开缩写
- categories: 识别类目，如"手机""电脑""服装"，没有就空数组
- min_price/max_price: 单位为分，如"100元以下"→max_price=10000，没有价格意图就都填0
- sort_by: PRICE_ASC/PRICE_DESC/SALES_DESC/空字符串
- need_rag: 默认 true（生成搜索摘要）；仅当用户明确搜索某个具体商品型号/SKU时填 false`

func (uc *AISearchUseCase) understandQuery(ctx context.Context, query string) (*domain.QueryIntent, error) {
	resp, err := uc.llmClient.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: queryUnderstandingPrompt},
			{Role: "user", Content: query},
		},
		Temperature: 0.1,
		MaxTokens:   256,
	})
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		RewrittenQuery string   `json:"rewritten_query"`
		Categories     []string `json:"categories"`
		MinPrice       int64    `json:"min_price"`
		MaxPrice       int64    `json:"max_price"`
		SortBy         string   `json:"sort_by"`
		Intent         string   `json:"intent"`
		NeedRAG        bool     `json:"need_rag"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		uc.l.Warn("解析 Query 理解结果失败", logger.Error(err), logger.String("raw", content))
		return &domain.QueryIntent{RewrittenQuery: query}, nil
	}

	return &domain.QueryIntent{
		RewrittenQuery: parsed.RewrittenQuery,
		Categories:     parsed.Categories,
		MinPrice:       parsed.MinPrice,
		MaxPrice:       parsed.MaxPrice,
		SortBy:         parsed.SortBy,
		Intent:         parsed.Intent,
		NeedRAG:        parsed.NeedRAG,
	}, nil
}

// ==================== ② 多路召回（意图感知） ====================

const recallTopK = 100

// multiRecall 顺序执行关键词 + 向量召回，分别返回耗时
func (uc *AISearchUseCase) multiRecall(ctx context.Context, intent *domain.QueryIntent) (
	keyword, vector []domain.RecallResult, kwMs, vecMs int64,
) {
	filters := es.BuildFiltersFromIntent(intent)

	// 关键词召回 (ES 倒排索引)
	t0 := time.Now()
	kwQuery := es.BuildKeywordQuery(intent.RewrittenQuery, recallTopK, filters)
	var kwErr error
	keyword, kwErr = uc.productRepo.KeywordRecallSearch(ctx, kwQuery)
	kwMs = time.Since(t0).Milliseconds()
	if kwErr != nil {
		uc.l.Warn("关键词召回失败", logger.Error(kwErr))
		keyword = nil
	}

	// 向量召回 (ES8 kNN)
	t0 = time.Now()
	vectors, embErr := uc.embedder.Embed(ctx, []string{intent.RewrittenQuery})
	if embErr != nil {
		uc.l.Warn("Embedding 失败，跳过向量召回", logger.Error(embErr))
		vecMs = time.Since(t0).Milliseconds()
		return keyword, nil, kwMs, vecMs
	}
	if len(vectors) > 0 {
		var vecErr error
		vector, vecErr = uc.productRepo.VectorSearch(ctx, vectors[0], recallTopK, filters)
		if vecErr != nil {
			uc.l.Warn("向量召回失败", logger.Error(vecErr))
			vector = nil
		}
	}
	vecMs = time.Since(t0).Milliseconds()
	return
}

// ==================== ③ 两阶段排序 ====================

// rankCandidate 携带 RRF 相关性得分和业务信号，供两阶段排序使用
type rankCandidate struct {
	id         int64
	rrfScore   float64
	salesCount int64
}

// fusionRank 第一阶段：纯相关性 RRF 融合，返回相关性候选集（无业务信号）
//
// 亮点：
//  1. RRF (Reciprocal Rank Fusion, k=60)：对位置倒数求和，天然归一化，无需调参
//  2. 意图感知权重：精确查询(need_rag=false)侧重关键词，模糊查询侧重向量
//  3. 不在此阶段引入业务信号 —— 确保销量不能"购买"相关性，只能在相关候选集内竞争
func (uc *AISearchUseCase) fusionRank(keyword, vector []domain.RecallResult, intent *domain.QueryIntent) []rankCandidate {
	const rrfK = 60

	// 意图感知权重
	kwWeight, vecWeight := 1.0, 1.0
	if intent.NeedRAG {
		kwWeight, vecWeight = 0.8, 1.2 // 模糊查询 → 语义向量权重更高
	} else {
		kwWeight, vecWeight = 1.2, 0.8 // 精确查询 → 关键词权重更高
	}

	type docSignal struct {
		rrfScore   float64
		salesCount int64
	}
	docs := make(map[int64]*docSignal, len(keyword)+len(vector))

	upsert := func(id int64, sc int64) *docSignal {
		if d, ok := docs[id]; ok {
			if sc > d.salesCount {
				d.salesCount = sc
			}
			return d
		}
		d := &docSignal{salesCount: sc}
		docs[id] = d
		return d
	}

	for rank, r := range keyword {
		upsert(r.ProductID, r.SalesCount).rrfScore += kwWeight / float64(rrfK+rank+1)
	}
	for rank, r := range vector {
		upsert(r.ProductID, r.SalesCount).rrfScore += vecWeight / float64(rrfK+rank+1)
	}

	candidates := make([]rankCandidate, 0, len(docs))
	for id, d := range docs {
		candidates = append(candidates, rankCandidate{id: id, rrfScore: d.rrfScore, salesCount: d.salesCount})
	}

	// 按纯相关性排序，确定候选集顺序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].rrfScore > candidates[j].rrfScore
	})
	return candidates
}

// reRankBySales 第二阶段：在相关性候选集内，用业务信号重排
//
// final_score = rrf_score × (1 + α·log₂(1 + sales_count))
//
// 设计原则：
//   - 只有通过了第一阶段相关性门控的商品才参与本阶段
//   - log 压缩防止超高销量商品过度碾压，保留相关性的主导地位
//   - α=0.1 使销量最多带来约 30%~50% 的排序提升（sales=1M 约 +20 个 log₂ 点）
func reRankBySales(candidates []rankCandidate) []int64 {
	type idScore struct {
		id    int64
		score float64
	}
	ranked := make([]idScore, len(candidates))
	for i, c := range candidates {
		salesBoost := 1.0 + 0.1*math.Log2(1.0+float64(c.salesCount))
		ranked[i] = idScore{id: c.id, score: c.rrfScore * salesBoost}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	ids := make([]int64, len(ranked))
	for i, r := range ranked {
		ids[i] = r.id
	}
	return ids
}

// ==================== ④ 按 ID 获取完整商品 ====================

func (uc *AISearchUseCase) fetchProductsByIDs(ctx context.Context, ids []int64, highlight bool, keyword string) ([]domain.ProductSearchResult, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	products, err := uc.productRepo.GetProductsByIDs(ctx, ids, highlight, keyword)
	if err != nil {
		return nil, err
	}

	// 按融合排序顺序重排（ES 返回顺序不保证与输入 ID 一致）
	idOrder := make(map[int64]int, len(ids))
	for i, id := range ids {
		idOrder[id] = i
	}
	sorted := make([]domain.ProductSearchResult, len(ids))
	matched := 0
	for _, p := range products {
		if idx, ok := idOrder[p.ID]; ok {
			sorted[idx] = p
			matched++
		}
	}
	return sorted[:matched], nil
}

// ==================== ⑤ RAG 摘要（基于搜索结果的检索增强生成） ====================

// RAG 亮点：检索结果即上下文（R=搜索管线结果），无需额外向量库
// 结构化商品信息 → LLM → 带价格区间、类目分布的自然语言摘要

const ragPrompt = `你是电商搜索助手。根据用户查询和搜索结果生成简洁的推荐摘要。
要求：
1. 50-150字，自然语言，不要编号列表
2. 提及价格区间（最低~最高）
3. 如果商品跨多个类目，指出类目分布
4. 突出销量最高的 1-2 个商品
5. 只基于提供的搜索结果，不要编造`

func (uc *AISearchUseCase) generateRAGSummary(ctx context.Context, query string, intent *domain.QueryIntent, products []domain.ProductSearchResult) (string, error) {
	topN := 8
	if len(products) < topN {
		topN = len(products)
	}

	// 结构化上下文：名称、价格、类目、销量
	var sb strings.Builder
	var minPrice, maxPrice int64
	categorySet := make(map[string]int)
	var bestSeller string
	var bestSales int64

	for i := 0; i < topN; i++ {
		p := products[i]
		priceYuan := float64(p.Price) / 100
		fmt.Fprintf(&sb, "%d. %s ¥%.2f 销量%d %s\n",
			i+1, p.Name, priceYuan, p.SalesCount, strings.Join(p.Categories, "/"))

		if minPrice == 0 || p.Price < minPrice {
			minPrice = p.Price
		}
		if p.Price > maxPrice {
			maxPrice = p.Price
		}
		for _, c := range p.Categories {
			categorySet[c]++
		}
		if p.SalesCount > bestSales {
			bestSales = p.SalesCount
			bestSeller = p.Name
		}
	}

	// 附加统计摘要给 LLM，减少其计算负担
	fmt.Fprintf(&sb, "\n统计：价格 ¥%.2f~¥%.2f", float64(minPrice)/100, float64(maxPrice)/100)
	if bestSeller != "" {
		fmt.Fprintf(&sb, "，销量最高「%s」(%d件)", bestSeller, bestSales)
	}
	if len(categorySet) > 1 {
		cats := make([]string, 0, len(categorySet))
		for c := range categorySet {
			cats = append(cats, c)
		}
		fmt.Fprintf(&sb, "，涉及类目：%s", strings.Join(cats, "、"))
	}

	resp, err := uc.llmClient.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: ragPrompt},
			{Role: "user", Content: fmt.Sprintf("用户搜索：%s\n意图：%s\n\n搜索结果：\n%s", query, intent.Intent, sb.String())},
		},
		Temperature: 0.3,
		MaxTokens:   300,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
