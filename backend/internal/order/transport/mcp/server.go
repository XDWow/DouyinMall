package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/XDWow/DouyinMall/backend/pkg/mcpruntime"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	orderservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
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
	client orderservice.Client
}

func NewServer(cfg Config, client orderservice.Client) (http.Handler, error) {
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
		case "query_order":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("order_id", mcpproto.Description("Order ID to query")),
				mcpproto.WithNumber("limit", mcpproto.Description("Maximum number of orders to return"), mcpproto.DefaultNumber(5)),
			), adapter.QueryOrder)
		}
	}

	return mcpserver.NewStreamableHTTPServer(
		server,
		mcpserver.WithHTTPContextFunc(mcpruntime.WithHTTPContext),
	), nil
}

func (a *Adapter) QueryOrder(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("missing runtime user_id"), nil
	}

	var args struct {
		OrderID int64 `json:"order_id"`
		Limit   int32 `json:"limit"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	if args.Limit <= 0 {
		args.Limit = 5
	}

	resp, err := a.client.ListOrder(ctx, &orderv1.ListOrderReq{
		UserId: runtime.UserID,
		Limit:  args.Limit,
	})
	if err != nil {
		return mcpproto.NewToolResultError("query order failed: " + err.Error()), nil
	}

	type orderSummary struct {
		OrderID     int64  `json:"order_id"`
		Status      string `json:"status"`
		TotalAmount int64  `json:"total_amount"`
		Currency    string `json:"currency"`
		CreatedAt   int64  `json:"created_at"`
	}
	items := make([]orderSummary, 0, len(resp.GetOrders()))
	for _, order := range resp.GetOrders() {
		if args.OrderID > 0 && order.GetOrderId() != args.OrderID {
			continue
		}
		items = append(items, orderSummary{
			OrderID:     order.GetOrderId(),
			Status:      order.GetOrderStatus().String(),
			TotalAmount: order.GetTotalAmount(),
			Currency:    order.GetCurrency(),
			CreatedAt:   order.GetCreatedAt(),
		})
	}
	if args.OrderID > 0 && len(items) == 0 {
		return mcpproto.NewToolResultError(fmt.Sprintf("order %d not found", args.OrderID)), nil
	}

	return mcpproto.NewToolResultText(toJSON(map[string]any{"orders": items})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "order-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{{
			Key:         "query_order",
			Name:        "query_order",
			Description: "Query the current user's orders.",
			Enabled:     true,
		}}
	}
	return cfg
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}


