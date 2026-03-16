package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/mcptools/tools"
	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
)

// InitMCPServer 创建 MCP Server 并注册全部工具
// 返回 mcp.MCPServer 接口，隐藏具体实现，方便切换实现和测试
func InitMCPServer(registry *tools.Registry) mcp.MCPServer {
	srv := mcp.NewServer("douyinmall-tools", "1.0.0")
	registry.Register(srv)
	return srv
}
