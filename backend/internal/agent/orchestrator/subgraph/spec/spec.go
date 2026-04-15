package spec

import (
	"strings"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

type Spec struct {
	SystemHint        string
	AllowedToolNames  []string
	AllowedSkillNames []string
	MaxRounds         int
	ToolMode          agenttool.ToolExecutionMode
	ReadOnly          bool
}

func (s Spec) ToolNames() []string {
	return normalizedNames(s.AllowedToolNames)
}

func (s Spec) SkillNames() []string {
	return normalizedNames(s.AllowedSkillNames)
}

func FilterAllowedSkillNames(names []string, reg *agentskill.Registry) []string {
	names = normalizedNames(names)
	if reg == nil || len(names) == 0 {
		return names
	}
	items := reg.Load(names)
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func normalizedNames(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, item := range input {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
