package global

import (
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

const (
	currentRefsKey        = "_current_refs"
	pendingSelectionsKey  = "_pending_selections"
)

func splitPersistedSessionState(persisted map[string]any) (map[string]any, domain.CurrentRefs, map[string]domain.PendingSelection) {
	slots := cloneAnyMap(persisted)
	currentRefs := domain.CurrentRefs{}
	pendingSelections := map[string]domain.PendingSelection{}
	if len(slots) == 0 {
		return nil, currentRefs, pendingSelections
	}

	if raw, ok := slots[currentRefsKey].(map[string]any); ok {
		currentRefs = parseCurrentRefs(raw)
	}
	delete(slots, currentRefsKey)

	if raw, ok := slots[pendingSelectionsKey].(map[string]any); ok {
		pendingSelections = parsePendingSelections(raw)
	}
	delete(slots, pendingSelectionsKey)
	if len(slots) == 0 {
		slots = nil
	}
	return slots, currentRefs, pendingSelections
}

func mergePersistedSessionState(slots map[string]any, currentRefs domain.CurrentRefs, pendingSelections map[string]domain.PendingSelection) map[string]any {
	merged := cloneAnyMap(slots)
	if merged == nil {
		merged = map[string]any{}
	}
	if currentRefs.ProductID != "" || currentRefs.OrderID != "" {
		merged[currentRefsKey] = map[string]any{
			"product_id": currentRefs.ProductID,
			"order_id":   currentRefs.OrderID,
		}
	}
	if len(pendingSelections) > 0 {
		raw := make(map[string]any, len(pendingSelections))
		for key, selection := range pendingSelections {
			entry := map[string]any{}
			if strings.TrimSpace(selection.Kind) != "" {
				entry["kind"] = selection.Kind
			}
			if len(selection.Options) > 0 {
				options := make(map[string]any, len(selection.Options))
				for optionKey, optionValue := range selection.Options {
					options[optionKey] = optionValue
				}
				entry["options"] = options
			}
			raw[key] = entry
		}
		merged[pendingSelectionsKey] = raw
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func applyTrustedRefsToSlots(slots map[string]any, currentRefs domain.CurrentRefs) {
	if slots == nil {
		return
	}
	if strings.TrimSpace(currentRefs.ProductID) != "" {
		slots["product_id"] = currentRefs.ProductID
	} else {
		delete(slots, "product_id")
	}
	if strings.TrimSpace(currentRefs.OrderID) != "" {
		slots["order_id"] = currentRefs.OrderID
	} else {
		delete(slots, "order_id")
	}
}

// promoteTrustedRefsIntoSlots 有 ref 则写入 product_id/order_id，不删已有键。
func promoteTrustedRefsIntoSlots(slots map[string]any, currentRefs domain.CurrentRefs) {
	if slots == nil {
		return
	}
	if strings.TrimSpace(currentRefs.ProductID) != "" {
		slots["product_id"] = currentRefs.ProductID
	}
	if strings.TrimSpace(currentRefs.OrderID) != "" {
		slots["order_id"] = currentRefs.OrderID
	}
}

func parseCurrentRefs(raw map[string]any) domain.CurrentRefs {
	return domain.CurrentRefs{
		ProductID: normalizeTrustedID(fmt.Sprint(raw["product_id"])),
		OrderID:   normalizeTrustedID(fmt.Sprint(raw["order_id"])),
	}
}

func parsePendingSelections(raw map[string]any) map[string]domain.PendingSelection {
	result := make(map[string]domain.PendingSelection, len(raw))
	for key, value := range raw {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		selection := domain.PendingSelection{
			Kind: strings.TrimSpace(fmt.Sprint(entry["kind"])),
		}
		if options, ok := entry["options"].(map[string]any); ok {
			selection.Options = make(map[string]string, len(options))
			for optionKey, optionValue := range options {
				text := strings.TrimSpace(fmt.Sprint(optionValue))
				if text != "" {
					selection.Options[optionKey] = text
				}
			}
		}
		result[key] = selection
	}
	return result
}

func clonePendingSelections(input map[string]domain.PendingSelection) map[string]domain.PendingSelection {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]domain.PendingSelection, len(input))
	for key, selection := range input {
		cloned := domain.PendingSelection{
			Kind: selection.Kind,
		}
		if len(selection.Options) > 0 {
			cloned.Options = make(map[string]string, len(selection.Options))
			for optionKey, optionValue := range selection.Options {
				cloned.Options[optionKey] = optionValue
			}
		}
		out[key] = cloned
	}
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func normalizeTrustedID(raw string) string {
	return support.DigitsOnlyID(raw)
}
