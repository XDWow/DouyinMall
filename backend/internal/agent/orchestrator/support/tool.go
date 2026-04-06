package support

import (
	"encoding/json"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func HydrateToolResults(state *orchestratorstate.ConversationState) {
	if state == nil {
		return
	}
	ss := orchestratorstate.EnsureSessionState(state)
	HydrateToolResultsIntoSlots(ss.Slots, state.ToolExecutions())
}

// HydrateToolResultsFromExecutions 保留给仍使用 ConversationState 的主流程节点。
func HydrateToolResultsFromExecutions(state *orchestratorstate.ConversationState, executions []domain.ToolExecution) {
	if state == nil {
		return
	}
	ss := orchestratorstate.EnsureSessionState(state)
	HydrateToolResultsIntoSlots(ss.Slots, executions)
}

// HydrateToolResultsIntoSlots 把工具执行结果显式写回槽位。
// 这样业务子图内部只需持有 slots，而不必临时构造 ConversationState。
func HydrateToolResultsIntoSlots(slots map[string]any, executions []domain.ToolExecution) {
	if slots == nil {
		return
	}
	root, _ := slots["tool_results"].(map[string]any)
	if root == nil {
		root = map[string]any{}
		slots["tool_results"] = root
	}
	for _, exec := range executions {
		var payload any
		if json.Unmarshal([]byte(exec.Result), &payload) != nil {
			payload = map[string]any{"raw": exec.Result}
		}
		root[exec.Name] = payload
	}
}

func ToolResultMap(state *orchestratorstate.ConversationState, toolName string) map[string]any {
	if state == nil {
		return nil
	}
	return ToolResultMapFromSlots(state.Session.Slots, toolName)
}

func ToolResultMapFromSlots(slots map[string]any, toolName string) map[string]any {
	if len(slots) == 0 {
		return nil
	}
	root, _ := slots["tool_results"].(map[string]any)
	if root == nil {
		return nil
	}
	result, _ := root[toolName].(map[string]any)
	return result
}

func ResetToolState(state *orchestratorstate.ConversationState) {
	if state == nil {
		return
	}
	state.Tool.Plans = nil
	state.Tool.CallMessage = nil
	state.Tool.ToolMessages = nil
}

func HasToolPlan(state *orchestratorstate.ConversationState, names ...string) bool {
	if state == nil || len(state.Tool.Plans) == 0 {
		return false
	}
	if len(names) == 0 {
		return len(state.Tool.Plans) > 0
	}
	for _, plan := range state.Tool.Plans {
		for _, name := range names {
			if strings.EqualFold(plan.Name, name) {
				return true
			}
		}
	}
	return false
}

func ToolResultRecord(state *orchestratorstate.ConversationState, toolName string) map[string]any {
	if state == nil {
		return nil
	}
	return ToolResultRecordFromSlots(state.Session.Slots, toolName)
}

func ToolResultRecordFromSlots(slots map[string]any, toolName string) map[string]any {
	result := ToolResultMapFromSlots(slots, toolName)
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
