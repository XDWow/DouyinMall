package support

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

func BaseQAAnswer(st *domain.State) string {
	if st != nil && st.Response != nil && len(st.Response.References) > 0 {
		item := st.Response.References[0]
		return FirstNonEmpty(item.Snippet, "我还需要更多上下文信息。")
	}
	return "我还需要更多上下文信息。"
}

func TemplateAnswer(st *domain.State) string {
	if st == nil {
		return "我还需要更多上下文信息。"
	}
	if st.Response != nil && strings.TrimSpace(st.Response.Reply) != "" {
		return st.Response.Reply
	}
	return BaseQAAnswer(st)
}

func NormalizeReply(reply string) string {
	reply = strings.TrimSpace(reply)
	reply = strings.Trim(reply, "\"")
	return reply
}

func EstimateConfidence(ctx context.Context, st *domain.State) float64 {
	if st == nil {
		return 0
	}
	score := 0.0
	if st.Response != nil {
		score = Clamp01(st.Response.Confidence) * 0.5
		if st.Response.NeedHandoff {
			score -= 0.25
		}
		if strings.TrimSpace(st.Response.Reply) != "" {
			score += 0.2
		}
	}
	if st.Response != nil && len(st.Response.References) > 0 {
		score += Clamp01(st.Response.References[0].Score) * 0.15
	}
	successTools := 0
	for _, exec := range agenttool.ToolExecutionsFromContext(ctx) {
		if exec.Success {
			successTools++
		}
	}
	if successTools > 0 {
		score += 0.15
	}
	return Clamp01(score)
}
