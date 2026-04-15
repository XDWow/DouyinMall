package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/mcpruntime"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Upstream UpstreamConfig `mapstructure:"upstream"` // 保留：网关或文档；进程内直连 UC 时不使用
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
	getOrder      *usecase.GetOrderUseCase
	listUserOrder *usecase.ListUserOrderUseCase
}

// NewServer 构建 MCP HTTP 处理器；get / list 分别走 GetOrder、ListUserOrder 用例。
func NewServer(cfg Config, getOrder *usecase.GetOrderUseCase, listUserOrder *usecase.ListUserOrderUseCase) (http.Handler, error) {
	cfg = applyDefaults(cfg)
	adapter := &Adapter{getOrder: getOrder, listUserOrder: listUserOrder}
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
		case "get_order":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("order_id", mcpproto.Description("订单 ID"), mcpproto.Required()),
			), adapter.GetOrder)
		case "list_user_orders", "query_order":
			// query_order 为历史配置键，与 list_user_orders 等价
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
			), adapter.ListUserOrders)
		}
	}

	return mcpserver.NewStreamableHTTPServer(
		server,
		mcpserver.WithHTTPContextFunc(mcpruntime.WithHTTPContext),
	), nil
}

func (a *Adapter) GetOrder(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("缺少运行上下文 user_id"), nil
	}

	var args getOrderArgs
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("参数无效: " + err.Error()), nil
	}
	if args.OrderID <= 0 {
		return mcpproto.NewToolResultError("order_id 必填且须为正整数"), nil
	}

	order, err := a.getOrder.Execute(ctx, usecase.GetOrderCmd{OrderID: args.OrderID})
	if err != nil {
		return mcpproto.NewToolResultError("查询订单失败: " + err.Error()), nil
	}
	if order.UserID != runtime.UserID {
		return mcpproto.NewToolResultError("订单不存在或无权查看"), nil
	}

	v, ok := domainOrderToItemView(order)
	if !ok {
		return mcpproto.NewToolResultError("订单数据异常"), nil
	}
	return mcpproto.NewToolResultText(marshalGetOrderPayload(getOrderPayload{Order: v})), nil
}

func (a *Adapter) ListUserOrders(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("缺少运行上下文 user_id"), nil
	}

	var args listUserOrdersArgs
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("参数无效: " + err.Error()), nil
	}

	cmd := toFirstPageListCmd(runtime.UserID)
	result, err := a.listUserOrder.Execute(cmd)
	if err != nil {
		return mcpproto.NewToolResultError("查询订单列表失败: " + err.Error()), nil
	}

	views := toQueryOrderItemViews(result.Orders)
	return mcpproto.NewToolResultText(marshalListUserOrdersPayload(listUserOrdersPayload{Orders: views})), nil
}

func applyDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "order-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []ToolConfig{
			{
				Key:         "get_order",
				Name:        "get_order",
				Description: "根据 order_id 查询单笔订单详情摘要；仅允许查询当前上下文用户本人的订单。",
				Enabled:     true,
			},
			{
				Key:         "list_user_orders",
				Name:        "list_user_orders",
				Description: "查询当前上下文用户订单列表第一页，固定最多 10 条。",
				Enabled:     true,
			},
		}
	}
	cfg.Tools = completeToolSet(cfg.Tools)
	return cfg
}

// completeToolSet 保证「单笔」与「列表」各至少有一个已启用的 tool。
// 旧配置只写 query_order（列表）时也会自动补上 get_order，避免 MCP 只暴露一个 tool。
func completeToolSet(tools []ToolConfig) []ToolConfig {
	hasGet, hasList := false, false
	for _, t := range tools {
		if !t.Enabled {
			continue
		}
		switch t.Key {
		case "get_order":
			hasGet = true
		case "list_user_orders", "query_order":
			hasList = true
		}
	}
	out := append([]ToolConfig(nil), tools...)
	if !hasGet {
		out = append(out, ToolConfig{
			Key:         "get_order",
			Name:        "get_order",
			Description: "根据 order_id 查询单笔订单详情摘要；仅允许查询当前上下文用户本人的订单。",
			Enabled:     true,
		})
	}
	if !hasList {
		out = append(out, ToolConfig{
			Key:         "list_user_orders",
			Name:        "list_user_orders",
			Description: "查询当前上下文用户订单列表第一页，固定最多 10 条。",
			Enabled:     true,
		})
	}
	return out
}
