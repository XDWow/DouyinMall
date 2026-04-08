package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/callback"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func TestWorkflowCallbackHandlerAppendsTraceStep(t *testing.T) {
	metrics := NewMetrics("test")
	handler := orchestratorcallback.Builder{
		Tracer:  noop.NewTracerProvider().Tracer("test"),
		Metrics: metrics,
	}.New()

	state := orchestratorstate.NewState(domain.ChatCommand{Message: "hello"}, nil, orchestratorstate.InitOptions{})
	info := &callbacks.RunInfo{Name: "IntentClassifyNode"}

	ctx := handler.OnStart(context.Background(), info, state)
	handler.OnEnd(ctx, info, state)

	if got := len(state.EnsureResponse().Trace.Steps); got != 1 {
		t.Fatalf("expected 1 trace step, got %d", got)
	}
	step := state.EnsureResponse().Trace.Steps[0]
	if step.Node != "IntentClassifyNode" {
		t.Fatalf("expected node IntentClassifyNode, got %s", step.Node)
	}
	if step.Status != "ok" {
		t.Fatalf("expected status ok, got %s", step.Status)
	}
}

func TestWorkflowCallbackHandlerRecordsError(t *testing.T) {
	metrics := NewMetrics("test")
	handler := orchestratorcallback.Builder{
		Tracer:  noop.NewTracerProvider().Tracer("test"),
		Metrics: metrics,
	}.New()

	state := orchestratorstate.NewState(domain.ChatCommand{Message: "hello"}, nil, orchestratorstate.InitOptions{})
	info := &callbacks.RunInfo{Name: "ToolsNode"}
	runErr := errors.New("tool failed")

	ctx := handler.OnStart(context.Background(), info, state)
	handler.OnError(ctx, info, runErr)

	if got := len(state.EnsureResponse().Trace.Steps); got != 1 {
		t.Fatalf("expected 1 trace step, got %d", got)
	}
	step := state.EnsureResponse().Trace.Steps[0]
	if step.Status != "error" {
		t.Fatalf("expected status error, got %s", step.Status)
	}
	if step.Detail != "tool failed" {
		t.Fatalf("expected detail tool failed, got %s", step.Detail)
	}
}
