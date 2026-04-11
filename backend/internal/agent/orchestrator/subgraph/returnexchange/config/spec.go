package config

import (
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	subgraphspec "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/spec"
)

var Spec = subgraphspec.Spec{
	SystemHint:        "",
	AllowedToolNames:  []string{"get_order", "list_user_orders", "query_order", "create_after_sale_request"},
	AllowedSkillNames: []string{"refund_return"},
	MaxRounds:         0,
	ToolMode:          agenttool.ToolExecutionSerial,
	ReadOnly:          false,
}
