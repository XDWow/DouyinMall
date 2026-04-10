package support

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
)

func ToolNamesForRoute(route domain.WorkflowRoute) []string {
	return append([]string(nil), subgraphmeta.Resolve(route).ToolNames...)
}
