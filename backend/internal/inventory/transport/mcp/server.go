package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	inventoryservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
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
	client inventoryservice.Client
}

func NewServer(cfg Config, client inventoryservice.Client) (http.Handler, error) {
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
		case "get_inventory":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("product_id", mcpproto.Description("Product ID"), mcpproto.Required()),
			), adapter.GetInventory)
		}
	}

	return mcpserver.NewStreamableHTTPServer(server), nil
}

func (a *Adapter) GetInventory(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	var args struct {
		ProductID int64 `json:"product_id"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	if args.ProductID <= 0 {
		return mcpproto.NewToolResultError("product_id is required"), nil
	}

	resp, err := a.client.GetInventory(ctx, &inventoryv1.GetInventoryReq{ProductId: args.ProductID})
	if err != nil {
		return mcpproto.NewToolResultError("get inventory failed: " + err.Error()), nil
	}

	return mcpproto.NewToolResultText(toJSON(map[string]any{
		"product_id":      resp.GetProductId(),
		"available_stock": resp.GetAvailableStock(),
		"sold_stock":      resp.GetSoldStock(),
	})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "inventory-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{{
			Key:         "get_inventory",
			Name:        "get_inventory",
			Description: "Get inventory stock by product ID.",
			Enabled:     true,
		}}
	}
	return cfg
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}


