package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	searchProductsUC  *usecase.SearchProductsUseCase
	searchMerchantsUC *usecase.SearchMerchantsUseCase
	suggestUC         *usecase.SuggestUseCase
	aggregationsUC    *usecase.AggregationsUseCase
	aiSearchUC        *usecase.AISearchUseCase
}

func NewHandler(
	searchProductsUC *usecase.SearchProductsUseCase,
	searchMerchantsUC *usecase.SearchMerchantsUseCase,
	suggestUC *usecase.SuggestUseCase,
	aggregationsUC *usecase.AggregationsUseCase,
	aiSearchUC *usecase.AISearchUseCase,
) *Handler {
	return &Handler{
		searchProductsUC:  searchProductsUC,
		searchMerchantsUC: searchMerchantsUC,
		suggestUC:         suggestUC,
		aggregationsUC:    aggregationsUC,
		aiSearchUC:        aiSearchUC,
	}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	group := engine.Group("/api/search")
	group.GET("/products", h.SearchProducts)
	group.GET("/merchants", h.SearchMerchants)
	group.GET("/suggest", h.Suggest)
	group.GET("/aggregations", h.Aggregations)
	group.GET("/ai/products", h.AISearchProducts)
}

func (h *Handler) SearchProducts(c *gin.Context) {
	resp, err := h.searchProductsUC.Execute(c.Request.Context(), &domain.SearchProductsReq{
		Keyword:         c.Query("keyword"),
		Page:            defaultInt64(c, "page", 1),
		PageSize:        defaultInt64(c, "page_size", 10),
		Categories:      queryStrings(c, "categories", "category"),
		MinPrice:        defaultInt64(c, "min_price", 0),
		MaxPrice:        defaultInt64(c, "max_price", 0),
		MerchantID:      defaultInt64(c, "merchant_id", 0),
		InStockOnly:     defaultBool(c, "in_stock_only", true),
		SortBy:          strings.TrimSpace(c.Query("sort_by")),
		EnableHighlight: defaultBool(c, "enable_highlight", true),
	})
	if err != nil {
		writeSearchError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"products":  resp.Products,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	}})
}

func (h *Handler) SearchMerchants(c *gin.Context) {
	verified := defaultBool(c, "verified_only", false)
	resp, err := h.searchMerchantsUC.Execute(c.Request.Context(), &domain.SearchMerchantsReq{
		Keyword:   c.Query("keyword"),
		Page:      defaultInt64(c, "page", 1),
		PageSize:  defaultInt64(c, "page_size", 10),
		Region:    c.Query("region"),
		MinRating: float32(defaultFloat64(c, "min_rating", 0)),
		Verified:  &verified,
		SortBy:    strings.TrimSpace(c.Query("sort_by")),
	})
	if err != nil {
		writeSearchError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"merchants": resp.Merchants,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	}})
}

func (h *Handler) Suggest(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "keyword is required"})
		return
	}

	limit := defaultInt64(c, "limit", 10)
	var (
		result []domain.SearchSuggestion
		err    error
	)
	if strings.EqualFold(c.Query("type"), "merchant") {
		result, err = h.suggestUC.MerchantSuggest(c.Request.Context(), keyword, limit)
	} else {
		result, err = h.suggestUC.ProductSuggest(c.Request.Context(), keyword, limit)
	}
	if err != nil {
		writeSearchError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"suggestions": result}})
}

func (h *Handler) Aggregations(c *gin.Context) {
	resp, err := h.aggregationsUC.Execute(c.Request.Context(), &domain.SearchProductsReq{
		Keyword:         c.Query("keyword"),
		Page:            defaultInt64(c, "page", 1),
		PageSize:        defaultInt64(c, "page_size", 10),
		Categories:      queryStrings(c, "categories", "category"),
		MinPrice:        defaultInt64(c, "min_price", 0),
		MaxPrice:        defaultInt64(c, "max_price", 0),
		MerchantID:      defaultInt64(c, "merchant_id", 0),
		InStockOnly:     defaultBool(c, "in_stock_only", true),
		SortBy:          strings.TrimSpace(c.Query("sort_by")),
		EnableHighlight: defaultBool(c, "enable_highlight", true),
	})
	if err != nil {
		writeSearchError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"categories":   resp.Categories,
		"price_ranges": resp.PriceRanges,
	}})
}

func (h *Handler) AISearchProducts(c *gin.Context) {
	query := strings.TrimSpace(c.Query("query"))
	if query == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "query is required"})
		return
	}

	resp, err := h.aiSearchUC.Execute(c.Request.Context(), &domain.AISearchProductsReq{
		Query:           query,
		Page:            defaultInt64(c, "page", 1),
		PageSize:        defaultInt64(c, "page_size", 10),
		EnableRAG:       defaultBool(c, "enable_rag", true),
		EnableHighlight: defaultBool(c, "enable_highlight", true),
	})
	if err != nil {
		writeSearchError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"products":     resp.Products,
		"total":        resp.Total,
		"page":         resp.Page,
		"page_size":    resp.PageSize,
		"query_intent": resp.QueryIntent,
		"rag_summary":  resp.RAGSummary,
		"metrics":      resp.Metrics,
	}})
}

func writeSearchError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		code = 4
	case errors.Is(err, domain.ErrSearchFailed), errors.Is(err, domain.ErrEmbeddingFail), errors.Is(err, domain.ErrLLMFailed):
		status = http.StatusBadGateway
	}
	c.JSON(status, ginx.Result{
		Code: code,
		Msg:  err.Error(),
	})
}

func defaultInt64(c *gin.Context, key string, fallback int64) int64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func defaultFloat64(c *gin.Context, key string, fallback float64) float64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func defaultBool(c *gin.Context, key string, fallback bool) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryStrings(c *gin.Context, primaryKey, secondaryKey string) []string {
	values := c.QueryArray(primaryKey)
	values = append(values, c.QueryArray(secondaryKey)...)
	if raw := strings.TrimSpace(c.Query(primaryKey)); raw != "" {
		values = append(values, strings.Split(raw, ",")...)
	}
	if raw := strings.TrimSpace(c.Query(secondaryKey)); raw != "" {
		values = append(values, strings.Split(raw, ",")...)
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
