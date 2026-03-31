package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1/searchservice"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1"
)

// SearchTool 商品搜索（ES 关键词检索）
type SearchTool struct {
	client searchservice.Client
}

func NewSearchTool(client searchservice.Client) *SearchTool {
	return &SearchTool{client: client}
}

type searchProductsArgs struct {
	Query    string `json:"query"`
	Page     int64  `json:"page"`
	PageSize int64  `json:"page_size"`
}

// SearchProducts 关键词搜索商品
func (t *SearchTool) SearchProducts(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[searchProductsArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.Query == "" {
		return mcp.NewErrorResult("query 不能为空")
	}
	if args.Page <= 0 {
		args.Page = 1
	}
	if args.PageSize <= 0 {
		args.PageSize = 10
	}

	resp, err := t.client.SearchProducts(ctx, &searchv1.SearchProductsReq{
		Keyword:  args.Query,
		Page:     args.Page,
		PageSize: args.PageSize,
	})
	if err != nil {
		return mcp.NewErrorResult("搜索商品失败: " + err.Error())
	}

	products := make([]map[string]any, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		products = append(products, map[string]any{
			"product_id":        p.GetId(),
			"name":              p.GetName(),
			"price":             formatCents(p.GetPrice()),
			"image_url":         p.GetPicture(),
			"short_description": truncate(p.GetDescription(), 80),
		})
	}

	if len(products) == 0 {
		return mcp.NewTextResult("未找到匹配的商品。")
	}
	return mcp.NewTextResult(toJSON(map[string]any{
		"products":  products,
		"total":     resp.GetTotal(),
		"page":      resp.GetPage(),
		"page_size": resp.GetPageSize(),
	}))
}

func formatCents(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
