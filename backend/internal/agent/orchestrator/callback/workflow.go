package callback

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type Builder struct {
	Tracer  trace.Tracer
	Metrics interface {
		ObserveNode(node, status string, d time.Duration)
	}
}

type stateKey struct{}

type callbackState struct {
	node      string
	startedAt time.Time
	state     *orchestratorstate.ConversationState
	span      trace.Span
}

var nodeNameSet = map[string]struct{}{
	"AccessGuardNode": {}, "SessionLoadNode": {}, "CachePolicyNode": {}, "MultiLevelCacheNode": {}, "PrepareIntentClassifyInputNode": {}, "IntentClassifyNode": {}, "ApplyIntentClassifyResultNode": {},
	"PrepareReturnPolicyRAGInputNode": {}, "ReturnPolicyRAGNode": {}, "ApplyReturnPolicyRAGResultNode": {},
	"SlotExtractNode": {}, "SlotCheckNode": {}, "AskUserNode": {},
	"OrderQueryGraph": {}, "InventoryGraph": {}, "ProductInfoGraph": {}, "AddToCartGraph": {}, "ReturnExchangeGraph": {}, "FallbackGraph": {},
	"PrepareProductRAGInputNode": {}, "ProductRAGNode": {}, "ApplyProductRAGResultNode": {},
	"PrepareFallbackRAGInputNode": {}, "FallbackRAGNode": {}, "ApplyFallbackRAGResultNode": {},
	"RouteNode": {}, "OrderQueryWorkflow": {}, "ReturnPolicyRAGWorkflow": {}, "InventoryWorkflow": {}, "ProductInfoWorkflow": {},
	"ReturnExchangeApplyWorkflow": {}, "FallbackWorkflow": {}, "BuildOrderQueryNode": {}, "CallOrderServiceNode": {},
	"OrderToolResultNode": {}, "BuildInventoryQueryNode": {}, "CallInventoryServiceNode": {}, "InventoryToolResultNode": {},
	"BuildProductInfoNode": {}, "CallProductServiceNode": {}, "ProductToolResultNode": {}, "GetOrderDetailNode": {},
	"CallReturnOrderServiceNode": {}, "ReturnOrderResultNode": {}, "EligibilityCheckNode": {}, "ConfirmSummaryNode": {}, "BuildAfterSaleSubmitNode": {},
	"CallAfterSaleServiceNode": {}, "SubmitAfterSaleNode": {},
	"FallbackResolveNode": {}, "PrepareSerialToolMessageNode": {},
	"PrepareParallelReadonlyToolMessageNode": {}, "ToolsNode": {}, "ApplyToolMessagesNode": {}, "ResponseRenderNode": {}, "CacheWritebackNode": {},
}

func (b Builder) New() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(b.onStart).
		OnEndFn(b.onEnd).
		OnErrorFn(b.onError).
		Build()
}

func (b Builder) onStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if !isWorkflowNode(info) {
		return ctx
	}
	state := callbackFlow(ctx, input)
	startedAt := time.Now()
	ctx, span := orchestratorobserve.StartSpan(ctx, b.Tracer, info.Name)

	orchestratorobserve.SendEvent(ctx, callbackStreamWriter(state), "node", map[string]any{"node": info.Name, "status": "start"})
	return context.WithValue(ctx, stateKey{}, callbackState{node: info.Name, startedAt: startedAt, state: state, span: span})
}

func (b Builder) onEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if isWorkflowNode(info) {
		b.finish(ctx, info, output, nil)
	}
	return ctx
}

func (b Builder) onError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if isWorkflowNode(info) {
		b.finish(ctx, info, nil, err)
	}
	return ctx
}

func (b Builder) finish(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput, err error) {
	st, _ := ctx.Value(stateKey{}).(callbackState)
	node := info.Name
	if node == "" {
		node = st.node
	}
	if node == "" {
		return
	}
	state := st.state
	if state == nil {
		state = callbackFlow(ctx, output)
	}
	latency := time.Duration(0)
	if !st.startedAt.IsZero() {
		latency = time.Since(st.startedAt)
	}
	status := "ok"
	detail := ""
	if err != nil {
		status = "error"
		detail = err.Error()
		if st.span != nil {
			st.span.RecordError(err)
			st.span.SetStatus(codes.Error, err.Error())
		}
	} else if st.span != nil {
		st.span.SetStatus(codes.Ok, "ok")
	}
	if st.span != nil {
		st.span.End()
	}
	orchestratorobserve.AppendTraceStep(state, node, status, latency, detail)
	if b.Metrics != nil {
		b.Metrics.ObserveNode(node, status, latency)
	}
	orchestratorobserve.SendEvent(ctx, callbackStreamWriter(state), "node", map[string]any{"node": node, "status": status, "latency_ms": latency.Milliseconds(), "detail": detail})
}

func isWorkflowNode(info *callbacks.RunInfo) bool {
	if info == nil {
		return false
	}
	_, ok := nodeNameSet[info.Name]
	return ok
}

func callbackFlow(ctx context.Context, value any) *orchestratorstate.ConversationState {
	if state, _ := value.(*orchestratorstate.ConversationState); state != nil {
		return state
	}
	if payload, _ := value.(map[string]any); payload != nil {
		if state, _ := payload["state"].(*orchestratorstate.ConversationState); state != nil {
			return state
		}
	}
	var state *orchestratorstate.ConversationState
	_ = compose.ProcessState[*orchestratorstate.ConversationState](ctx, func(_ context.Context, state *orchestratorstate.ConversationState) error {
		if state != nil {
			state = state
		}
		return nil
	})
	return state
}

func callbackStreamWriter(state *orchestratorstate.ConversationState) orchestratorstate.StreamWriter {
	if state == nil {
		return nil
	}
	return state.StreamWriter
}
