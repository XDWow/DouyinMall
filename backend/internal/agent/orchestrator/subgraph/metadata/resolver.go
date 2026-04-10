// Package metadata：按 Route 聚合子图 tool/skill 白名单。
package metadata

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
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

func Resolve(route domain.WorkflowRoute) Whitelist {
	switch route {
	case domain.RouteOrderQuery:
		return Whitelist{ToolNames: orderquerymeta.AllowedToolNames(), SkillNames: orderquerymeta.AllowedSkillNames()}
	case domain.RouteInventory:
		return Whitelist{ToolNames: inventorymeta.AllowedToolNames(), SkillNames: inventorymeta.AllowedSkillNames()}
	case domain.RouteProductInfo:
		return Whitelist{ToolNames: productinfometa.AllowedToolNames(), SkillNames: productinfometa.AllowedSkillNames()}
	case domain.RouteAddToCart:
		return Whitelist{ToolNames: addtocartmeta.AllowedToolNames(), SkillNames: addtocartmeta.AllowedSkillNames()}
	case domain.RouteReturnPolicy:
		return Whitelist{ToolNames: returnpolicymeta.AllowedToolNames(), SkillNames: returnpolicymeta.AllowedSkillNames()}
	case domain.RouteReturnExchangeApply:
		return Whitelist{ToolNames: returnexchangemeta.AllowedToolNames(), SkillNames: returnexchangemeta.AllowedSkillNames()}
	default:
		return Whitelist{ToolNames: fallbackmeta.AllowedToolNames(), SkillNames: fallbackmeta.AllowedSkillNames()}
	}
}

func FilteredSkillNames(route domain.WorkflowRoute, reg *agentskill.Registry) []string {
	names := Resolve(route).SkillNames
	if reg == nil {
		return append([]string(nil), names...)
	}
	items := reg.Load(names)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}
