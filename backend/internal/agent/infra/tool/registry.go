package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Registry holds the MCP tools that are actually available to the agent service.
// We keep only two capabilities here:
// 1. look up tools by whitelist name
// 2. expose one shared ToolsNode for execution
type Registry struct {
	tools map[string]einotool.BaseTool
	node  *compose.ToolsNode
}

type registeredTool struct {
	baseTool einotool.BaseTool
	info     *schema.ToolInfo
}

func newRegistry(ctx context.Context, registered []registeredTool) (*Registry, error) {
	tools := make([]einotool.BaseTool, 0, len(registered))
	toolMap := make(map[string]einotool.BaseTool, len(registered))

	for _, item := range registered {
		if item.baseTool == nil || item.info == nil || strings.TrimSpace(item.info.Name) == "" {
			continue
		}
		tools = append(tools, item.baseTool)
		toolMap[item.info.Name] = item.baseTool
	}

	// The whole service shares one sequential ToolsNode.
	// Deterministic nodes build tool_calls themselves.
	// ADK agent nodes let the model produce tool_calls.
	// Both paths end up executing here.
	node, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               tools,
		ExecuteSequentially: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create tools node: %w", err)
	}

	return &Registry{
		tools: toolMap,
		node:  node,
	}, nil
}

func (r *Registry) ToolsNode() (*compose.ToolsNode, error) {
	if r == nil {
		return nil, fmt.Errorf("tool registry is not initialized")
	}
	if r.node == nil {
		return nil, fmt.Errorf("tools node is not initialized")
	}
	return r.node, nil
}

// Tools returns the whitelisted business tools in input order.
func (r *Registry) Tools(names []string) []einotool.BaseTool {
	if r == nil || len(r.tools) == 0 || len(names) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(names))
	out := make([]einotool.BaseTool, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if tool := r.tools[name]; tool != nil {
			out = append(out, tool)
		}
	}
	return out
}

func (r *Registry) Has(name string) bool {
	if r == nil || r.tools == nil {
		return false
	}
	_, ok := r.tools[strings.TrimSpace(name)]
	return ok
}
