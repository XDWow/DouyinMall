package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client MCP HTTP 客户端
// 连接远程 MCP Server，发现工具并执行调用
type Client struct {
	endpoint   string
	httpClient *http.Client
	serverInfo ServerInfo

	mu    sync.RWMutex
	tools []Tool // tools/list 缓存
	idSeq int64
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Endpoint string        // MCP Server HTTP 地址，如 http://localhost:9090/mcp
	Timeout  time.Duration // 单次 RPC 超时，默认 10s
}

// NewClient 创建 MCP 客户端并执行 initialize 握手
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	c := &Client{
		endpoint:   cfg.Endpoint,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}

	// initialize 握手
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo:      ClientInfo{Name: "douyinmall-agent", Version: "1.0.0"},
	}

	var initResult InitializeResult
	if err := c.call(ctx, "initialize", params, &initResult); err != nil {
		return nil, fmt.Errorf("MCP initialize 失败: %w", err)
	}
	c.serverInfo = initResult.ServerInfo

	// 发送 initialized 通知（无需响应）
	_ = c.notify(ctx, "notifications/initialized")

	// 拉取工具列表
	if err := c.refreshTools(ctx); err != nil {
		return nil, fmt.Errorf("MCP tools/list 失败: %w", err)
	}

	return c, nil
}

// Tools 返回当前 MCP Server 提供的工具列表
func (c *Client) Tools() []Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]Tool, len(c.tools))
	copy(cp, c.tools)
	return cp
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, fmt.Errorf("MCP tools/call [%s] 失败: %w", name, err)
	}
	return &result, nil
}

// ==================== 内部方法 ====================

func (c *Client) refreshTools(ctx context.Context) error {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return err
	}
	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	c.idSeq++
	req := Request{
		JSONRPC: "2.0",
		ID:      c.idSeq,
		Method:  method,
	}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var rpcResp Response
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("decode result: %w", err)
		}
	}
	return nil
}

func (c *Client) notify(ctx context.Context, method string) error {
	req := Request{JSONRPC: "2.0", Method: method}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
