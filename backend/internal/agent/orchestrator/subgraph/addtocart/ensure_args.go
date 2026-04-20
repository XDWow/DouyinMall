package addtocart

import (
	"context"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func ensureAddToCartArgs(ctx context.Context, resolved ResolvedAddToCart) (ResolvedAddToCart, error) {
	wasInterrupted, hasState, interrupted := compose.GetInterruptState[domain.AddToCartInterruptState](ctx)
	isResumeFlow, hasData, resumeMap := compose.GetResumeContext[map[string]any](ctx)

	state := domain.AddToCartInterruptState{
		ProductID: strings.TrimSpace(resolved.ProductID),
		Spec:      strings.TrimSpace(resolved.Spec),
		Quantity:  resolved.Quantity,
	}
	if wasInterrupted && hasState {
		state = interrupted
	}
	if wasInterrupted && !isResumeFlow {
		missing := computeMissingFields(state)
		state.MissingFields = missing
		return ResolvedAddToCart{}, compose.StatefulInterrupt(ctx, clarificationInfo(missing), state)
	}
	if hasData {
		state = mergeResumeInto(state, domain.ResumeDataFromMap(resumeMap))
	}

	missing := computeMissingFields(state)
	if len(missing) == 0 {
		return ResolvedAddToCart{
			ProductID: strings.TrimSpace(state.ProductID),
			Spec:      strings.TrimSpace(state.Spec),
			Quantity:  state.Quantity,
		}, nil
	}

	state.MissingFields = missing
	return ResolvedAddToCart{}, compose.StatefulInterrupt(ctx, clarificationInfo(missing), state)
}

func mergeResumeInto(st domain.AddToCartInterruptState, rd domain.ResumeData) domain.AddToCartInterruptState {
	for key, value := range rd.Fields {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "product", "product_id":
			st.ProductID = strings.TrimSpace(value)
		case "product_name":
			st.ProductName = strings.TrimSpace(value)
		case "spec":
			st.Spec = strings.TrimSpace(value)
		case "quantity":
			if q, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && q > 0 {
				st.Quantity = q
			}
		}
	}
	return st
}

func computeMissingFields(st domain.AddToCartInterruptState) []string {
	var missing []string
	if strings.TrimSpace(st.ProductID) == "" {
		missing = append(missing, "product")
	}
	if strings.TrimSpace(st.Spec) == "" {
		missing = append(missing, "spec")
	}
	if st.Quantity <= 0 {
		missing = append(missing, "quantity")
	}
	return missing
}

func clarificationInfo(missing []string) map[string]any {
	question := "Please provide the missing information."
	if len(missing) > 0 {
		switch missing[0] {
		case "product":
			question = "Which product do you want to add to cart?"
		case "spec":
			question = "Which spec do you want?"
		case "quantity":
			question = "How many do you want to add?"
		}
	}
	return map[string]any{
		"type":           "clarification",
		"question":       question,
		"missing_fields": append([]string(nil), missing...),
	}
}
