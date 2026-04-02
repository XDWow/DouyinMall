package tool

import (
	"fmt"
	"sync"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
)

type Registry struct {
	tools          []einotool.BaseTool
	invokables     map[string]registeredInvokableTool
	sequentialNode *compose.ToolsNode
	parallelNode   *compose.ToolsNode
}

func (r *Registry) Tools() []einotool.BaseTool {
	return r.tools
}

func (r *Registry) ToolsNode(mode ToolExecutionMode) (*compose.ToolsNode, error) {
	if r == nil {
		return nil, fmt.Errorf("tool registry is not initialized")
	}
	switch mode {
	case ToolExecutionParallelReadOnly:
		if r.parallelNode == nil {
			return nil, fmt.Errorf("parallel tools node is not initialized")
		}
		return r.parallelNode, nil
	default:
		if r.sequentialNode == nil {
			return nil, fmt.Errorf("sequential tools node is not initialized")
		}
		return r.sequentialNode, nil
	}
}

func (r *Registry) Policy(name string) (ToolPolicy, bool) {
	if r == nil || r.invokables == nil {
		return ToolPolicy{}, false
	}
	item, ok := r.invokables[name]
	if !ok {
		return ToolPolicy{}, false
	}
	return item.policy, true
}

func (r *Registry) ValidatePlans(plans []dto.ToolCallPlan, mode ToolExecutionMode) error {
	if len(plans) == 0 || mode != ToolExecutionParallelReadOnly {
		return nil
	}
	for _, plan := range plans {
		policy, ok := r.Policy(plan.Name)
		if !ok {
			return fmt.Errorf("tool not registered: %s", plan.Name)
		}
		if !policy.ReadOnly || policy.RequiresOrdering {
			return fmt.Errorf("tool %s cannot run in parallel readonly mode", plan.Name)
		}
	}
	return nil
}

type SafeExecutionRecorder struct {
	mu    sync.Mutex
	items []dto.ToolExecution
}

func NewSafeExecutionRecorder() *SafeExecutionRecorder {
	return &SafeExecutionRecorder{}
}

func (r *SafeExecutionRecorder) Record(exec dto.ToolExecution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, exec)
}

func (r *SafeExecutionRecorder) Snapshot() []dto.ToolExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]dto.ToolExecution, len(r.items))
	copy(out, r.items)
	return out
}
