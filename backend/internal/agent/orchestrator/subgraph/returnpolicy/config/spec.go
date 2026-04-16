package config

import (
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	subgraphspec "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/spec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

var Spec = subgraphspec.Spec{
	SystemHint:        prompt.SubgraphSystemReturnPolicy,
	AllowedToolNames:  nil,
	AllowedSkillNames: []string{"return_policy_qa"},
	MaxRounds:         8,
	ToolMode:          agenttool.ToolExecutionSerial,
	ReadOnly:          true,
}
