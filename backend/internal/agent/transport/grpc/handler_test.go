package grpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentusecase "github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
)

func TestToProtoChatResponse(t *testing.T) {
	resp := toProtoChatResponse(&agentusecase.ChatOutput{
		Reply:          "Order found successfully",
		Intent:         domain.IntentOrderQuery,
		NeedHandoff:    true,
		HandoffReason:  "low_confidence",
		References:     []agentusecase.KnowledgeRef{{ID: "k1", Title: "Order status", Snippet: "Order is being processed", Category: "faq", Score: 0.91}},
		ToolExecutions: []agentusecase.ToolExecution{{Name: "query_order", Arguments: map[string]any{"order_id": 123}, Success: true, Result: "ok", LatencyMs: 33}},
		Trace:          agentusecase.Trace{TraceID: "trace_nested", CheckpointID: "cp_123", RewrittenQuery: "query order 123"},
		TraceID:        "trace_top",
		SessionID:      "sess_1",
		Interrupt:      &agentusecase.InterruptInfo{CheckpointID: "cp_interrupt", RerunNodes: []string{"InterruptNode"}},
	}, "check order 123")

	require.Equal(t, "Order found successfully", resp.GetReply())
	require.Equal(t, "sess_1", resp.GetSessionId())
	require.Equal(t, "trace_top", resp.GetTraceId())
	require.Equal(t, "cp_interrupt", resp.GetCheckpointId())
	require.Equal(t, []string{"InterruptNode"}, resp.GetRerunNodes())
	require.NotEmpty(t, resp.GetSuggestedQuestions())
	require.Len(t, resp.GetKnowledge(), 1)
	require.Len(t, resp.GetToolExecs(), 1)
	require.NotNil(t, resp.GetHandoff())
	require.Equal(t, "low_confidence", resp.GetHandoff().GetEscalationReason())
	require.Equal(t, "neutral", detectEmotion("this is confusing"))
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2026, 4, 1, 12, 30, 0, 0, time.Local)
	require.Equal(t, "2026-04-01 12:30:00", formatTime(ts))
}
