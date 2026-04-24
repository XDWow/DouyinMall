package addtocart

import (
	"context"
	"strings"

	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
)

func resolveInvoke(_ context.Context, in AddToCartResolveInput) (ResolvedAddToCart, error) {
	out := ResolvedAddToCart{
		ProductID: strings.TrimSpace(in.ProductID),
		SKUID:     strings.TrimSpace(in.SKUID),
		Spec:      strings.TrimSpace(in.Spec),
		Quantity:  in.Quantity,
	}
	if out.ProductID == "" {
		out.ProductID = subgraphcommon.ResolveSelection(in.ProductRef, in.CurrentProduct, in.ProductList)
	}
	// 都没指定，那就默认cur
	if out.ProductID == "" && strings.TrimSpace(in.ProductRef) == "" {
		out.ProductID = strings.TrimSpace(in.CurrentProduct)
	}
	if out.Spec == "" {
		out.Spec = strings.TrimSpace(in.CurrentSpec)
	}
	return out, nil
}
