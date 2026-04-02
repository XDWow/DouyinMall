package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	productservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
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
	client productservice.Client
}

func NewServer(cfg Config, client productservice.Client) (http.Handler, error) {
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
		case "get_product":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("product_id", mcpproto.Description("Product ID"), mcpproto.Required()),
			), adapter.GetProduct)
		}
	}

	return mcpserver.NewStreamableHTTPServer(server), nil
}

func (a *Adapter) GetProduct(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	var args struct {
		ProductID int64 `json:"product_id"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	if args.ProductID <= 0 {
		return mcpproto.NewToolResultError("product_id is required"), nil
	}

	resp, err := a.client.GetProducts(ctx, &productv1.GetProductsReq{Id: []int64{args.ProductID}})
	if err != nil {
		return mcpproto.NewToolResultError("get product failed: " + err.Error()), nil
	}
	products := resp.GetProduct()
	if len(products) == 0 || products[0] == nil {
		return mcpproto.NewToolResultError("product not found"), nil
	}
	product := products[0]

	return mcpproto.NewToolResultText(toJSON(map[string]any{
		"product": map[string]any{
			"id":            product.GetId(),
			"name":          product.GetName(),
			"description":   product.GetDescription(),
			"picture":       product.GetPicture(),
			"price":         product.GetPrice(),
			"currency":      product.GetCurrency(),
			"categories":    product.GetCategories(),
			"merchant_name": product.GetMerchantName(),
			"in_stock":      product.GetInStock(),
		},
	})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "product-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{{
			Key:         "get_product",
			Name:        "get_product",
			Description: "Get product details by product ID.",
			Enabled:     true,
		}}
	}
	return cfg
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
