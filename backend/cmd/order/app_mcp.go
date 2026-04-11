package main

import (
	"fmt"
	"net/http"

	ordermcp "github.com/XDWow/DouyinMall/backend/internal/order/transport/mcp"
)

// OrderMCPHandler 按 MCP 配置构造 HTTP 处理器；用例仅在 wire 与 transport 之间传递，不作为 App 的公开字段暴露。
func (a *App) OrderMCPHandler(cfg ordermcp.Config) (http.Handler, error) {
	if a == nil || a.getOrderUC == nil || a.listUserOrderUC == nil {
		return nil, fmt.Errorf("订单 MCP 依赖未注入")
	}
	return ordermcp.NewServer(cfg, a.getOrderUC, a.listUserOrderUC)
}
