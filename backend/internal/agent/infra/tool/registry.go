package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Registry 保存已注册工具，并暴露一套统一的 ToolsNode
type Registry struct {
	toolInfos map[string]*schema.ToolInfo
	node      *compose.ToolsNode
}

type registeredTool struct {
	baseTool einotool.BaseTool
	info     *schema.ToolInfo
}

func newRegistry(ctx context.Context, registered []registeredTool) (*Registry, error) {
	tools := make([]einotool.BaseTool, 0, len(registered))
	toolInfos := make(map[string]*schema.ToolInfo, len(registered))

	for _, item := range registered {
		if item.baseTool == nil || item.info == nil || strings.TrimSpace(item.info.Name) == "" {
			continue
		}
		tools = append(tools, item.baseTool)
		toolInfos[item.info.Name] = item.info
	}

	// 整个服务只保留一套顺序 ToolsNode：
	// 确定性节点自己构造 tool_calls，Agent 子图由模型产出 tool_calls，
	// 两条路径最终都统一落到这里执行
	node, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               tools,
		ExecuteSequentially: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create tools node: %w", err)
	}

	return &Registry{
		toolInfos: toolInfos,
		node:      node,
	}, nil
}

func (r *Registry) ToolInfos(names []string) []*schema.ToolInfo {
	if r == nil || len(r.toolInfos) == 0 || len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]*schema.ToolInfo, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if info := r.toolInfos[name]; info != nil {
			out = append(out, info)
		}
	}
	return out
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

func (r *Registry) Has(name string) bool {
	if r == nil || r.toolInfos == nil {
		return false
	}
	_, ok := r.toolInfos[strings.TrimSpace(name)]
	return ok
}
