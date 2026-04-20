package addtocart

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

func InputFromState(st *domain.State) (AddToCartResolveInput, error) {
	if st == nil || st.Session == nil {
		return AddToCartResolveInput{}, fmt.Errorf("state session is required")
	}

	quantity := 0
	if raw := sharednode.SlotString(st.Session.Slots, "quantity"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
			quantity = parsed
		}
	}

	return AddToCartResolveInput{
		ProductID:      sharednode.SlotString(st.Session.Slots, "product_id"),
		ProductName:    sharednode.SlotString(st.Session.Slots, "product_name"),
		ProductRef:     sharednode.SlotString(st.Session.Slots, "product_ref"),
		Spec:           sharednode.SlotString(st.Session.Slots, "spec"),
		Quantity:       quantity,
		CurrentProduct: strings.TrimSpace(st.Session.CurrentProduct),
		CurrentSpec:    strings.TrimSpace(st.Session.CurrentSpec),
		ProductList:    append([]string(nil), st.Session.ProductList...),
	}, nil
}
