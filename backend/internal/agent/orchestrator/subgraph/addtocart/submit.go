package addtocart

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

func submitAddToCart(
	ctx context.Context,
	resolved ResolvedAddToCart,
	registry *agenttool.Registry,
) (*domain.ChatResult, error) {
	if strings.TrimSpace(resolved.ProductID) == "" {
		return nil, fmt.Errorf("product_id is required")
	}

	result := &domain.ChatResult{Intent: domain.IntentAddToCart}
	if registry == nil || !registry.Has("add_to_cart") {
		result.Reply = "Cart service is currently unavailable. Please try again later."
		result.NeedHandoff = true
		result.HandoffReason = "cart_service_unavailable"
		return result, nil
	}

	callMessage, err := support.BuildToolCallMessage("add_to_cart", map[string]any{
		"product_id": resolved.ProductID,
		"quantity":   resolved.Quantity,
	})
	if err != nil {
		return nil, err
	}

	toolsNode, err := registry.ToolsNode()
	if err != nil {
		return nil, err
	}
	if _, err := toolsNode.Invoke(ctx, callMessage); err != nil {
		return nil, err
	}

	toolResult := subgraphcommon.LatestToolResultMap(ctx, "add_to_cart")
	if ok, exists := support.ToolResultBool(toolResult, "success"); exists && ok {
		_ = domain.ProcessState(ctx, func(st *domain.State) error {
			if st == nil || st.Session == nil {
				return nil
			}
			st.Session.CurrentProduct = strings.TrimSpace(resolved.ProductID)
			st.Session.CurrentSpec = strings.TrimSpace(resolved.Spec)
			return nil
		})
	}
	if strings.TrimSpace(result.Reply) == "" {
		result.Reply = fmt.Sprintf("Added product %s to cart.", resolved.ProductID)
	}
	return result, nil
}
