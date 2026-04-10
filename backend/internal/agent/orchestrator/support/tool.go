package support

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func HydrateToolResults(st *domain.State) {
	if st == nil {
		return
	}
	ss := &st.Session
	HydrateToolResultsIntoSlots(ss.Slots, st.ToolExecutions())
	HydrateCurrentRefs(ss)
}

// HydrateToolResultsFromExecutions 保留给主流程节点在已有执行结果时复用。
func HydrateToolResultsFromExecutions(st *domain.State, executions []domain.ToolExecution) {
	if st == nil {
		return
	}
	ss := &st.Session
	HydrateToolResultsIntoSlots(ss.Slots, executions)
	HydrateCurrentRefs(ss)
}

// HydrateToolResultsIntoSlots 把工具执行结果显式写回槽位。
// 这样业务子图内部只需持有 slots，而不必临时构造主流程状态。
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

func ToolResultMap(st *domain.State, toolName string) map[string]any {
	if st == nil {
		return nil
	}
	return ToolResultMapFromSlots(st.Session.Slots, toolName)
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

func ResetToolState(st *domain.State) {
	if st == nil {
		return
	}
	st.Tool.Plans = nil
	st.Tool.CallMessage = nil
	st.Tool.ToolMessages = nil
}

// CollectUsedToolNames 从本轮 Plan 与 Recorder 快照收敛工具名（无参数、无结果），供出口组装与缓存策略。
func CollectUsedToolNames(st *domain.State) []string {
	if st == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, plan := range st.Tool.Plans {
		add(plan.Name)
	}
	for _, exec := range st.ToolExecutions() {
		add(exec.Name)
	}
	return out
}

func HasToolPlan(st *domain.State, names ...string) bool {
	if st == nil || len(st.Tool.Plans) == 0 {
		return false
	}
	if len(names) == 0 {
		return len(st.Tool.Plans) > 0
	}
	for _, plan := range st.Tool.Plans {
		for _, name := range names {
			if strings.EqualFold(plan.Name, name) {
				return true
			}
		}
	}
	return false
}

func ToolResultRecord(st *domain.State, toolName string) map[string]any {
	if st == nil {
		return nil
	}
	return ToolResultRecordFromSlots(st.Session.Slots, toolName)
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

func HydrateCurrentRefs(session *domain.Session) {
	if session == nil || len(session.Slots) == 0 {
		return
	}
	if id := firstTrustedID(
		ToolResultRecordFromSlots(session.Slots, "get_product"),
		ToolResultMapFromSlots(session.Slots, "get_product"),
	); id != "" {
		session.CurrentRefs.ProductID = id
	}
	if id := firstTrustedID(
		ToolResultRecordFromSlots(session.Slots, "query_order"),
		ToolResultRecordFromSlots(session.Slots, "get_order"),
		ToolResultMapFromSlots(session.Slots, "query_order"),
	); id != "" {
		session.CurrentRefs.OrderID = id
	}
	if session.CurrentRefs.ProductID != "" {
		session.Slots["product_id"] = session.CurrentRefs.ProductID
	}
	if session.CurrentRefs.OrderID != "" {
		session.Slots["order_id"] = session.CurrentRefs.OrderID
	}
}

func firstTrustedID(records ...map[string]any) string {
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		for _, key := range []string{"product_id", "order_id", "id"} {
			if id := digitsOnly(fmt.Sprint(record[key])); id != "" {
				return id
			}
		}
	}
	return ""
}

func digitsOnly(raw string) string {
	var builder strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
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

// SelectedToolNames 收敛出当前轮真正相关的工具名称。
// 有 plan / execution 时以运行期结果为准；否则回退到子图白名单。
func SelectedToolNames(st *domain.State) []string {
	if st == nil {
		return nil
	}
	seen := make(map[string]struct{})
	names := make([]string, 0, len(st.Tool.Plans)+len(st.ToolExecutions()))
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for _, plan := range st.Tool.Plans {
		appendName(plan.Name)
	}
	for _, exec := range st.ToolExecutions() {
		appendName(exec.Name)
	}
	if len(names) > 0 {
		return names
	}
	for _, name := range ToolNamesForRoute(st.Session.Route) {
		appendName(name)
	}
	return names
}
