package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptranport "github.com/mark3labs/mcp-go/client/transport"
	mcpproto "github.com/mark3labs/mcp-go/mcp"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/mcpruntime"
)

type discoveredMCPTool struct {
	ServerName string
	Tool       *mcpWrappedTool
}

func NewMCPRegistry(ctx context.Context, servers []agentconfig.MCPServerConfig) (*Registry, error) {
	discovered, err := discoverMCPTools(ctx, servers)
	if err != nil {
		return nil, err
	}

	registered := make([]registeredTool, 0, len(discovered)+1)
	for _, item := range discovered {
		registered = append(registered, registeredTool{
			baseTool: item.Tool,
			info:     item.Tool.info,
		})
	}
	return newRegistry(ctx, registered)
}

func discoverMCPTools(ctx context.Context, servers []agentconfig.MCPServerConfig) ([]discoveredMCPTool, error) {
	discovered := make([]discoveredMCPTool, 0)
	seen := make(map[string]string)
	serverErrors := make([]string, 0)

	for _, cfg := range servers {
		if !cfg.Enabled {
			continue
		}
		serverName := strings.TrimSpace(cfg.Name)
		endpoint := strings.TrimSpace(cfg.Endpoint)
		if serverName == "" || endpoint == "" {
			continue
		}

		client, err := newOfficialMCPClient(ctx, serverName, endpoint, secondsOrDefault(cfg.TimeoutSeconds, 10*time.Second))
		if err != nil {
			serverErrors = append(serverErrors, err.Error())
			continue
		}

		tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: client})
		if err != nil {
			serverErrors = append(serverErrors, fmt.Sprintf("discover MCP tools from %s failed: %v", serverName, err))
			continue
		}

		serverTools := make([]discoveredMCPTool, 0, len(tools))
		for _, baseTool := range tools {
			invokable, ok := baseTool.(einotool.InvokableTool)
			if !ok {
				return nil, fmt.Errorf("mcp tool from %s is not invokable", serverName)
			}
			info, err := baseTool.Info(ctx)
			if err != nil {
				return nil, fmt.Errorf("load mcp tool info from %s failed: %w", serverName, err)
			}
			if info == nil || strings.TrimSpace(info.Name) == "" {
				return nil, fmt.Errorf("mcp tool from %s returned empty tool info", serverName)
			}
			if existing, ok := seen[info.Name]; ok {
				return nil, fmt.Errorf("duplicate MCP tool %s from %s and %s", info.Name, existing, serverName)
			}
			seen[info.Name] = serverName

			serverTools = append(serverTools, discoveredMCPTool{
				ServerName: serverName,
				Tool: &mcpWrappedTool{
					serverName: serverName,
					info:       info,
					inner:      invokable,
				},
			})
		}

		slices.SortFunc(serverTools, func(a, b discoveredMCPTool) int {
			return strings.Compare(a.Tool.info.Name, b.Tool.info.Name)
		})
		discovered = append(discovered, serverTools...)
	}

	if len(discovered) == 0 {
		if len(serverErrors) > 0 {
			return nil, fmt.Errorf("no MCP tools discovered: %s", strings.Join(serverErrors, "; "))
		}
		return nil, fmt.Errorf("no MCP tools discovered")
	}
	return discovered, nil
}

func newOfficialMCPClient(ctx context.Context, serverName, endpoint string, timeout time.Duration) (mcpclient.MCPClient, error) {
	httpTransport, err := mcptranport.NewStreamableHTTP(
		endpoint,
		mcptranport.WithHTTPTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create MCP transport for %s failed: %w", serverName, err)
	}

	client := mcpclient.NewClient(httpTransport)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("start MCP client for %s failed: %w", serverName, err)
	}

	initReq := mcpproto.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpproto.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpproto.Implementation{
		Name:    "douyinmall-agent",
		Version: "1.0.0",
	}
	if _, err := client.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("initialize MCP client for %s failed: %w", serverName, err)
	}
	return client, nil
}

type mcpWrappedTool struct {
	serverName string
	info       *schema.ToolInfo
	inner      einotool.InvokableTool
}

func (t *mcpWrappedTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *mcpWrappedTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	start := time.Now()
	runtime := runtimeFromContext(ctx)
	headers := mcpruntime.Headers(mcpruntime.Runtime{
		UserID:    runtime.UserID,
		SessionID: runtime.SessionID,
		TraceID:   runtime.TraceID,
	})

	callOpts := append([]einotool.Option{}, opts...)
	if len(headers) > 0 {
		callOpts = append(callOpts, mcpp.WithCustomHeaders(headers))
	}

	rawResult, err := t.inner.InvokableRun(ctx, argumentsInJSON, callOpts...)
	exec := domain.ToolExecution{
		Name:       t.info.Name,
		Arguments:  parseToolArguments(argumentsInJSON),
		Success:    err == nil,
		LatencyMs:  time.Since(start).Milliseconds(),
		OccurredAt: start,
		Metadata:   map[string]string{"mcp_server": t.serverName},
	}
	if err != nil {
		exec.Error = parseMCPErrorText(err)
		if recorder := executionRecorderFromContext(ctx); recorder != nil {
			recorder.Record(exec)
		}
		return "", err
	}

	text := extractMCPResultText(rawResult)
	exec.Result = text
	if recorder := executionRecorderFromContext(ctx); recorder != nil {
		recorder.Record(exec)
	}
	return text, nil
}

func extractMCPResultText(raw string) string {
	var result mcpproto.CallToolResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return strings.TrimSpace(raw)
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		text := strings.TrimSpace(mcpproto.GetTextFromContent(content))
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func parseMCPErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return extractMCPResultText(text[start : end+1])
	}
	return text
}

func secondsOrDefault(raw int, fallback time.Duration) time.Duration {
	if raw <= 0 {
		return fallback
	}
	return time.Duration(raw) * time.Second
}

func parseToolArguments(argumentsInJSON string) map[string]any {
	if strings.TrimSpace(argumentsInJSON) == "" {
		return nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return map[string]any{"raw": argumentsInJSON}
	}
	return args
}
