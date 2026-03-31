package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Server MCP HTTP 服务端
// 实现 initialize / tools/list / tools/call 三个 JSON-RPC 方法
// 通过 Streamable HTTP 传输（POST /mcp）
type Server struct {
	info  ServerInfo
	mu    sync.RWMutex
	tools map[string]toolEntry
	order []string // 保持注册顺序
}

type toolEntry struct {
	def     Tool
	handler ToolHandler
}

// NewServer 创建 MCP Server
func NewServer(name, version string) *Server {
	return &Server{
		info:  ServerInfo{Name: name, Version: version},
		tools: make(map[string]toolEntry),
	}
}

// RegisterTool 注册一个工具
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tools[tool.Name]; !exists {
		s.order = append(s.order, tool.Name)
	}
	s.tools[tool.Name] = toolEntry{def: tool, handler: handler}
}

// Tools 返回已注册的工具列表，保持注册顺序
func (s *Server) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Tool, 0, len(s.order))
	for _, name := range s.order {
		list = append(list, s.tools[name].def)
	}
	return list
}

// CallTool 直接调用工具，不经过 HTTP 层
func (s *Server) CallTool(ctx context.Context, name string, arguments json.RawMessage) *CallToolResult {
	s.mu.RLock()
	entry, ok := s.tools[name]
	s.mu.RUnlock()
	if !ok {
		return NewErrorResult(fmt.Sprintf("tool not found: %s", name))
	}
	return entry.handler(ctx, arguments)
}

// ServeHTTP 实现 http.Handler，处理 POST /mcp 请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, CodeParseError, "invalid JSON: "+err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, CodeInvalidRequest, "jsonrpc must be 2.0")
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, req)
	case "tools/list":
		writeRPCResult(w, req.ID, ListToolsResult{Tools: s.Tools()})
	case "tools/call":
		s.handleCallTool(w, r, req)
	default:
		writeRPCError(w, req.ID, CodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(w http.ResponseWriter, req Request) {
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		ServerInfo:      s.info,
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
	}
	writeRPCResult(w, req.ID, result)
}

func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request, req Request) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, CodeInvalidParams, "invalid params: "+err.Error())
		return
	}
	result := s.CallTool(r.Context(), params.Name, params.Arguments)
	writeRPCResult(w, req.ID, result)
}

// ==================== 响应写入 ====================

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	data, _ := json.Marshal(result)
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
