package domain

import (
	"fmt"
	"strings"
)

// ResumeData is the unified interrupt/resume payload
type ResumeData struct {
	Approved bool              `json:"approved,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
}

func ResumeDataFromMap(m map[string]any) ResumeData {
	var rd ResumeData
	if len(m) == 0 {
		return rd
	}
	if v, ok := m["approved"]; ok {
		rd.Approved = coerceAnyToBool(v)
	}
	if raw, ok := m["fields"]; ok {
		switch typed := raw.(type) {
		case map[string]any:
			rd.Fields = make(map[string]string, len(typed))
			for k, v := range typed {
				rd.Fields[k] = strings.TrimSpace(fmt.Sprint(v))
			}
		case map[string]string:
			rd.Fields = CloneStringStringMap(typed)
		}
	}
	return rd
}

func coerceAnyToBool(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		s := strings.TrimSpace(strings.ToLower(typed))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return false
	}
}

func CloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func CloneStringStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func ParseApprovedFromResumeMap(m map[string]any) (bool, bool) {
	if len(m) == 0 {
		return false, false
	}
	value, ok := m["approved"]
	if !ok {
		return false, false
	}
	return coerceAnyToBool(value), true
}

func (rd ResumeData) MergeFieldsIntoSlots(slots map[string]any) {
	if len(rd.Fields) == 0 || slots == nil {
		return
	}
	for key, value := range rd.Fields {
		slots[key] = value
	}
}
