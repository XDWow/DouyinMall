//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/mcptools/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/mcptools/tools"
	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	"github.com/google/wire"
)

func InitMCPServer() mcp.MCPServer {
	wire.Build(
		// 下游 Kitex 客户端
		ioc.InitSearchClient,
		ioc.InitProductClient,
		ioc.InitCartClient,
		ioc.InitCheckoutClient,
		ioc.InitOrderClient,

		// 工具实现
		tools.NewSearchTool,
		tools.NewProductTool,
		tools.NewCartTool,
		tools.NewCheckoutTool,
		tools.NewOrderTool,

		// 工具注册表
		tools.NewRegistry,

		// MCP Server
		ioc.InitMCPServer,
	)
	return nil
}
