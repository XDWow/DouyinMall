package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1"
	searchservice "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1/searchservice"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Upstream UpstreamConfig `mapstructure:"upstream"`
	Tools    []ToolConfig   `mapstructure:"tools"`
}

type ServerConfig struct {
	Addr    string `mapstructure:"addr"`
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type UpstreamConfig struct {
	ServiceName string `mapstructure:"service_name"`
	DirectAddr  string `mapstructure:"direct_addr"`
}

type ToolConfig struct {
	Key         string `mapstructure:"key"`
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Enabled     bool   `mapstructure:"enabled"`
}

type Adapter struct {
	client searchservice.Client
}

func NewServer(cfg Config, client searchservice.Client) (http.Handler, error) {
	cfg = applyDefaults(cfg)
	adapter := &Adapter{client: client}
	server := mcpserver.NewMCPServer(
		cfg.Server.Name,
		cfg.Server.Version,
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	for _, tool := range cfg.Tools {
		if !tool.Enabled {
			continue
		}
		switch tool.Key {
		case "search_product":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithString("query", mcpproto.Description("Product search keyword"), mcpproto.Required()),
				mcpproto.WithNumber("limit", mcpproto.Description("Maximum number of products to return"), mcpproto.DefaultNumber(5)),
			), adapter.SearchProduct)
		}
	}

	return mcpserver.NewStreamableHTTPServer(server), nil
}

func (a *Adapter) SearchProduct(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int64  `json:"limit"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	if strings.TrimSpace(args.Query) == "" {
		return mcpproto.NewToolResultError("query is required"), nil
	}
	if args.Limit <= 0 {
		args.Limit = 5
	}

	resp, err := a.client.SearchProducts(ctx, &searchv1.SearchProductsReq{
		Keyword:         args.Query,
		Page:            1,
		PageSize:        args.Limit,
		EnableHighlight: true,
	})
	if err != nil {
		return mcpproto.NewToolResultError("search product failed: " + err.Error()), nil
	}

	type productSummary struct {
		ID           int64    `json:"id"`
		Name         string   `json:"name"`
		Price        int64    `json:"price"`
		Categories   []string `json:"categories,omitempty"`
		MerchantName string   `json:"merchant_name,omitempty"`
		InStock      bool     `json:"in_stock"`
	}
	items := make([]productSummary, 0, len(resp.GetProducts()))
	for _, product := range resp.GetProducts() {
		items = append(items, productSummary{
			ID:           product.GetId(),
			Name:         product.GetName(),
			Price:        product.GetPrice(),
			Categories:   product.GetCategories(),
			MerchantName: product.GetMerchantName(),
			InStock:      product.GetInStock(),
		})
	}
	return mcpproto.NewToolResultText(toJSON(map[string]any{"products": items})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "search-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{{
			Key:         "search_product",
			Name:        "search_product",
			Description: "Search products by keyword.",
			Enabled:     true,
		}}
	}
	return cfg
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
