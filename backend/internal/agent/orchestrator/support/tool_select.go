package support

import (
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
)

// ToolNamesForRoute 只是读取对应子图维护的白名单，不在 support 层硬编码。
func ToolNamesForRoute(route orchestratorstate.WorkflowRoute) []string {
	return append([]string(nil), subgraphmeta.Resolve(route).ToolNames...)
}
