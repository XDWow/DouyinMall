// Package mcp 实现 Model Context Protocol (MCP) 核心类型。
// 参考规范：https://modelcontextprotocol.io/specification
// 仅实现 Tools 子集（initialize + tools/list + tools/call），满足 AI Agent tool calling 场景。
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
)

// ToolHandler 工具执行函数签名
// ctx 携带请求上下文（含 user_id 等元信息），arguments 是 LLM 传入的 JSON 参数
type ToolHandler func(ctx context.Context, arguments json.RawMessage) *CallToolResult

type MCPServer interface {
	Tools() []Tool
	CallTool(ctx context.Context, name string, arguments json.RawMessage) *CallToolResult
	http.Handler
}

// MCPClient 是 Agent 依赖的 MCP 客户端接口。
// 面向接口编程，使得：
//  1. 测试时可注入 Mock，不需要启动真实 MCP Server
//  2. 未来可替换为官方 go-sdk 的 ClientSession，只需实现此接口
//  3. ioc 层返回 nil 表示"纯 RAG 降级模式"，接口类型的 nil 语义更明确
type MCPClient interface {
	// 返回 MCP Server 当前声明的工具列表（已缓存，初始化时拉取）
	Tools() []Tool
	// CallTool 调用指定工具，arguments 为 JSON 序列化的入参
	CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error)
}

// ==================== JSON-RPC 2.0 ====================

// Request JSON-RPC 2.0 请求
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response JSON-RPC 2.0 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError JSON-RPC 错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// 标准 JSON-RPC 错误码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ==================== MCP 协议消息 ====================

// InitializeParams initialize 请求参数
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      ClientInfo `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult initialize 响应
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListToolsResult tools/list 响应
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams tools/call 请求参数
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock MCP 内容块（text 类型）
type ContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ==================== 工厂函数 ====================

// NewTextResult 构造成功的文本结果
func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// NewErrorResult 构造错误结果
func NewErrorResult(msg string) *CallToolResult {
	return &CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// MustJSON 将任意值序列化为 json.RawMessage，序列化失败 panic
func MustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("mcp.MustJSON: " + err.Error())
	}
	return b
}

// ProtocolVersion 当前实现的 MCP 协议版本
const ProtocolVersion = "2024-11-05"
