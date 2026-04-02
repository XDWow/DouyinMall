package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/XDWow/DouyinMall/backend/pkg/mcpruntime"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
	cartservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
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
	client cartservice.Client
}

func NewServer(cfg Config, client cartservice.Client) (http.Handler, error) {
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
		case "add_to_cart":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("product_id", mcpproto.Description("Product ID"), mcpproto.Required()),
				mcpproto.WithNumber("quantity", mcpproto.Description("Quantity to add"), mcpproto.DefaultNumber(1)),
			), adapter.AddToCart)
		}
	}

	return mcpserver.NewStreamableHTTPServer(
		server,
		mcpserver.WithHTTPContextFunc(mcpruntime.WithHTTPContext),
	), nil
}

func (a *Adapter) AddToCart(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("missing runtime user_id"), nil
	}

	var args struct {
		ProductID int64 `json:"product_id"`
		Quantity  int64 `json:"quantity"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	if args.ProductID <= 0 {
		return mcpproto.NewToolResultError("product_id is required"), nil
	}
	if args.Quantity <= 0 {
		args.Quantity = 1
	}

	productIDs := make([]int64, 0, args.Quantity)
	for i := int64(0); i < args.Quantity; i++ {
		productIDs = append(productIDs, args.ProductID)
	}
	if _, err := a.client.AddItem(ctx, &cartv1.AddItemReq{
		UserId:     runtime.UserID,
		ProductIds: productIDs,
	}); err != nil {
		return mcpproto.NewToolResultError("add to cart failed: " + err.Error()), nil
	}

	return mcpproto.NewToolResultText(toJSON(map[string]any{
		"success":    true,
		"product_id": args.ProductID,
		"quantity":   args.Quantity,
	})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "cart-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{{
			Key:         "add_to_cart",
			Name:        "add_to_cart",
			Description: "Add a product into the current user's cart.",
			Enabled:     true,
		}}
	}
	return cfg
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
