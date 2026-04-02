package support

import (
	"encoding/json"
	"strings"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

func HydrateToolResults(flow *orchestratorstate.FlowContext) {
	if flow == nil {
		return
	}
	state := orchestratorstate.EnsureSessionState(flow)
	if state.Slots == nil {
		state.Slots = map[string]any{}
	}
	if state.Slots["tool_results"] == nil {
		state.Slots["tool_results"] = map[string]any{}
	}
	target, _ := state.Slots["tool_results"].(map[string]any)
	for _, exec := range flow.ToolExecutions() {
		var payload any
		if json.Unmarshal([]byte(exec.Result), &payload) != nil {
			payload = map[string]any{"raw": exec.Result}
		}
		target[exec.Name] = payload
	}
}

func ToolResultMap(flow *orchestratorstate.FlowContext, toolName string) map[string]any {
	if flow == nil {
		return nil
	}
	root, _ := flow.State.Slots["tool_results"].(map[string]any)
	if root == nil {
		return nil
	}
	result, _ := root[toolName].(map[string]any)
	return result
}

func ResetToolDecision(flow *orchestratorstate.FlowContext) {
	if flow == nil {
		return
	}
	flow.Tool.Plans = nil
	flow.Tool.DecisionMessage = nil
	flow.Tool.ToolMessages = nil
}

func HasToolPlan(flow *orchestratorstate.FlowContext, names ...string) bool {
	if flow == nil || len(flow.Tool.Plans) == 0 {
		return false
	}
	if len(names) == 0 {
		return len(flow.Tool.Plans) > 0
	}
	for _, plan := range flow.Tool.Plans {
		for _, name := range names {
			if strings.EqualFold(plan.Name, name) {
				return true
			}
		}
	}
	return false
}

func ToolResultRecord(flow *orchestratorstate.FlowContext, toolName string) map[string]any {
	result := ToolResultMap(flow, toolName)
	if len(result) == 0 {
		return nil
	}
	if item, ok := result["order"].(map[string]any); ok && len(item) > 0 {
		return item
	}
	if items, ok := result["orders"].([]any); ok && len(items) > 0 {
		if item, ok := items[0].(map[string]any); ok && len(item) > 0 {
			return item
		}
	}
	return result
}

func ToolResultBool(record map[string]any, keys ...string) (bool, bool) {
	if len(record) == 0 {
		return false, false
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "1", "yes", "y", "supported", "allow", "allowed", "eligible":
				return true, true
			case "false", "0", "no", "n", "unsupported", "deny", "denied", "ineligible":
				return false, true
			}
		case float64:
			return typed != 0, true
		case int:
			return typed != 0, true
		case int64:
			return typed != 0, true
		}
	}
	return false, false
}
