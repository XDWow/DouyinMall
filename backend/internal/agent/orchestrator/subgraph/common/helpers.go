package common

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

func HistoryLines(turns []domain.MessageTurn) []string {
	if len(turns) == 0 {
		return nil
	}
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		out = append(out, string(turn.Role)+": "+content)
	}
	return out
}

func FindToolResult(execs []domain.ToolExecution, name string) map[string]any {
	for i := len(execs) - 1; i >= 0; i-- {
		if strings.TrimSpace(execs[i].Name) != strings.TrimSpace(name) {
			continue
		}
		if strings.TrimSpace(execs[i].Result) == "" {
			return nil
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(execs[i].Result), &payload); err != nil {
			return nil
		}
		return payload
	}
	return nil
}

func LatestToolResultMap(ctx context.Context, name string) map[string]any {
	return FindToolResult(agenttool.ToolExecutionsFromContext(ctx), name)
}
