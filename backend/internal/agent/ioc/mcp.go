package ioc

import (
	"log"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	"github.com/spf13/viper"
)

// InitMCPClient 初始化 MCP 工具客户端
// 如果配置中未设置 mcp.endpoint 或连接失败，返回 nil（降级为纯 RAG 模式）
func InitMCPClient() mcp.MCPClient {
	endpoint := viper.GetString("mcp.endpoint")
	if endpoint == "" {
		log.Println("[MCP] mcp.endpoint 未配置，工具调用不可用，降级为纯 RAG 模式")
		return nil
	}

	timeout := viper.GetDuration("mcp.timeout")
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	client, err := mcp.NewClient(mcp.ClientConfig{
		Endpoint: endpoint,
		Timeout:  timeout,
	})
	if err != nil {
		log.Printf("[MCP] 连接 MCP Server 失败（%s），工具调用不可用: %v\n", endpoint, err)
		return nil
	}

	log.Printf("[MCP] 已连接 MCP Server: %s，可用工具 %d 个\n", endpoint, len(client.Tools()))
	return client
}
