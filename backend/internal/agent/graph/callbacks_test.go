package graph

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/graph/callback"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

func TestWorkflowCallbackHandlerAppendsTraceStep(t *testing.T) {
	metrics := NewMetrics("test")
	handler := orchestratorcallback.Builder{
		Tracer:  noop.NewTracerProvider().Tracer("test"),
		Metrics: metrics,
	}.New()

	flow := orchestratorstate.NewFlowContext(dto.ChatRequest{Message: "hello"}, nil, orchestratorstate.InitOptions{})
	info := &callbacks.RunInfo{Name: "IntentClassifyChain"}

	ctx := handler.OnStart(context.Background(), info, flow)
	handler.OnEnd(ctx, info, flow)

	if got := len(flow.EnsureResponse().Trace.Steps); got != 1 {
		t.Fatalf("expected 1 trace step, got %d", got)
	}
	step := flow.EnsureResponse().Trace.Steps[0]
	if step.Node != "IntentClassifyChain" {
		t.Fatalf("expected node IntentClassifyChain, got %s", step.Node)
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

	flow := orchestratorstate.NewFlowContext(dto.ChatRequest{Message: "hello"}, nil, orchestratorstate.InitOptions{})
	info := &callbacks.RunInfo{Name: "ToolsNode"}
	runErr := errors.New("tool failed")

	ctx := handler.OnStart(context.Background(), info, flow)
	handler.OnError(ctx, info, runErr)

	if got := len(flow.EnsureResponse().Trace.Steps); got != 1 {
		t.Fatalf("expected 1 trace step, got %d", got)
	}
	step := flow.EnsureResponse().Trace.Steps[0]
	if step.Status != "error" {
		t.Fatalf("expected status error, got %s", step.Status)
	}
	if step.Detail != "tool failed" {
		t.Fatalf("expected detail tool failed, got %s", step.Detail)
	}
}
