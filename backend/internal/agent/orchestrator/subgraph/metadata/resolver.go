package metadata

import (
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	addtocartmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart/metadata"
	fallbackmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback/metadata"
	inventorymeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory/metadata"
	orderquerymeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery/metadata"
	productinfometa "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo/metadata"
	returnexchangemeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange/metadata"
	returnpolicymeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy/metadata"
)

type Whitelist struct {
	ToolNames  []string
	SkillNames []string
}

func Resolve(route orchestratorstate.WorkflowRoute) Whitelist {
	switch route {
	case orchestratorstate.RouteOrderQuery:
		return Whitelist{ToolNames: orderquerymeta.AllowedToolNames(), SkillNames: orderquerymeta.AllowedSkillNames()}
	case orchestratorstate.RouteInventory:
		return Whitelist{ToolNames: inventorymeta.AllowedToolNames(), SkillNames: inventorymeta.AllowedSkillNames()}
	case orchestratorstate.RouteProductInfo:
		return Whitelist{ToolNames: productinfometa.AllowedToolNames(), SkillNames: productinfometa.AllowedSkillNames()}
	case orchestratorstate.RouteAddToCart:
		return Whitelist{ToolNames: addtocartmeta.AllowedToolNames(), SkillNames: addtocartmeta.AllowedSkillNames()}
	case orchestratorstate.RouteReturnPolicy:
		return Whitelist{ToolNames: returnpolicymeta.AllowedToolNames(), SkillNames: returnpolicymeta.AllowedSkillNames()}
	case orchestratorstate.RouteReturnExchangeApply:
		return Whitelist{ToolNames: returnexchangemeta.AllowedToolNames(), SkillNames: returnexchangemeta.AllowedSkillNames()}
	default:
		return Whitelist{ToolNames: fallbackmeta.AllowedToolNames(), SkillNames: fallbackmeta.AllowedSkillNames()}
	}
}
