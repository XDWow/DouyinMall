package node

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
)

type OrderReadNode struct{ suite *Suite }
type InventoryReadNode struct{ suite *Suite }
type ProductInfoNode struct{ suite *Suite }

func (s *Suite) OrderRead() *OrderReadNode         { return &OrderReadNode{suite: s} }
func (s *Suite) InventoryRead() *InventoryReadNode { return &InventoryReadNode{suite: s} }
func (s *Suite) ProductInfo() *ProductInfoNode     { return &ProductInfoNode{suite: s} }

func (n *OrderReadNode) NormalizeIntent(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	flow.State.ReadOnly = true
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *OrderReadNode) BuildQuery(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "query_order") {
		flow.State.FinalAnswer = "Order query service is unavailable. Handing off to a human agent."
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "order_service_unavailable"
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	plans := []dto.ToolCallPlan{{Name: "query_order", Arguments: map[string]any{"limit": 5}}}
	if orderID := graphstate.SlotString(flow, "order_id"); orderID != "" {
		if value, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			plans[0].Arguments["order_id"] = value
			plans[0].Arguments["limit"] = 1
		}
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, flow, plans)
}

func (n *OrderReadNode) ApplyResult(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	support.HydrateToolResults(flow)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *InventoryReadNode) BuildQuery(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "get_inventory") {
		flow.State.FinalAnswer = "Inventory query service is unavailable. Handing off to a human agent."
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "inventory_service_unavailable"
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	productID, err := n.suite.ToolExec().ParseSlotInt64(flow, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, flow, []dto.ToolCallPlan{{Name: "get_inventory", Arguments: map[string]any{"product_id": productID}}})
}

func (n *InventoryReadNode) ApplyResult(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	support.HydrateToolResults(flow)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ProductInfoNode) SplitIntent(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if support.IsAdvisoryProductInfo(flow.State.RawQuery) {
		graphstate.SetSlot(flow, "product_mode", "advisory")
	} else {
		graphstate.SetSlot(flow, "product_mode", "factual")
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ProductInfoNode) BuildQuery(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "get_product") {
		flow.State.FinalAnswer = "Product service is unavailable. Handing off to a human agent."
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "product_service_unavailable"
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	productID, err := n.suite.ToolExec().ParseSlotInt64(flow, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}
	plans := []dto.ToolCallPlan{{Name: "get_product", Arguments: map[string]any{"product_id": productID}}}
	if n.suite.deps.Hooks.RegistryHasTool != nil && n.suite.deps.Hooks.RegistryHasTool(ctx, "get_inventory") && support.MentionsInventory(flow.State.RawQuery) {
		plans = append(plans, dto.ToolCallPlan{Name: "get_inventory", Arguments: map[string]any{"product_id": productID}})
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, flow, plans)
}

func (n *ProductInfoNode) ApplyResult(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	support.HydrateToolResults(flow)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}
