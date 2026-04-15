package config

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	addtocartconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart/config"
	fallbackconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback/config"
	inventoryconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory/config"
	orderqueryconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery/config"
	productinfoconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo/config"
	returnexchangeconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange/config"
	returnpolicyconfig "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy/config"
	subgraphspec "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/spec"
)

type Whitelist struct {
	ToolNames  []string
	SkillNames []string
}

func Resolve(route domain.WorkflowRoute) Whitelist {
	switch route {
	case domain.RouteOrderQuery:
		return whitelistFromSpec(orderqueryconfig.Spec)
	case domain.RouteInventory:
		return whitelistFromSpec(inventoryconfig.Spec)
	case domain.RouteProductInfo:
		return whitelistFromSpec(productinfoconfig.Spec)
	case domain.RouteAddToCart:
		return whitelistFromSpec(addtocartconfig.Spec)
	case domain.RouteReturnPolicy:
		return whitelistFromSpec(returnpolicyconfig.Spec)
	case domain.RouteReturnExchangeApply:
		return whitelistFromSpec(returnexchangeconfig.Spec)
	default:
		return whitelistFromSpec(fallbackconfig.Spec)
	}
}

func whitelistFromSpec(spec subgraphspec.Spec) Whitelist {
	return Whitelist{
		ToolNames:  spec.ToolNames(),
		SkillNames: spec.SkillNames(),
	}
}
