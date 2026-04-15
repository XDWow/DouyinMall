package config

import (
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	subgraphspec "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/spec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

var Spec = subgraphspec.Spec{
	SystemHint:        prompt.SubgraphSystemAddToCart,
	AllowedToolNames:  []string{"get_product", "add_to_cart"},
	AllowedSkillNames: nil,
	MaxRounds:         8,
	ToolMode:          agenttool.ToolExecutionSerial,
	ReadOnly:          false,
}
