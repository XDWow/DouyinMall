package callback

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/graph/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
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
	flow      *orchestratorstate.FlowContext
	span      trace.Span
}

var nodeNameSet = map[string]struct{}{
	"AccessGuardNode": {}, "SessionLoadNode": {}, "L0ExactCacheNode": {}, "IntentClassifyChain": {}, "BuildIntentPromptInputNode": {},
	"IntentPromptNode": {}, "IntentModelNode": {}, "ApplyIntentNode": {}, "SlotExtractNode": {}, "SlotCheckNode": {}, "AskUserNode": {},
	"RouteNode": {}, "OrderQueryWorkflow": {}, "ReturnPolicyRAGWorkflow": {}, "InventoryWorkflow": {}, "ProductInfoWorkflow": {},
	"ReturnExchangeApplyWorkflow": {}, "FallbackWorkflow": {}, "NormalizeOrderIntentNode": {}, "BuildOrderQueryNode": {}, "CallOrderServiceNode": {},
	"OrderToolResultNode": {}, "BuildInventoryQueryNode": {}, "CallInventoryServiceNode": {}, "InventoryToolResultNode": {},
	"ProductInfoIntentSplitNode": {}, "BuildProductInfoNode": {}, "CallProductServiceNode": {}, "ProductToolResultNode": {}, "GetOrderDetailNode": {},
	"CallReturnOrderServiceNode": {}, "ReturnOrderResultNode": {}, "EligibilityCheckNode": {}, "ConfirmSummaryNode": {}, "BuildAfterSaleSubmitNode": {},
	"CallAfterSaleServiceNode": {}, "SubmitAfterSaleNode": {},
	"FallbackResolveNode": {}, "QueryRewriteNode": {}, "RewriteEvaluateNode": {}, "RewriteIdentityNode": {}, "BuildRewritePromptInputNode": {},
	"RewritePromptNode": {}, "RewriteModelNode": {}, "ApplyRewriteNode": {}, "KnowledgeRetrieverNode": {}, "PrepareRetrieveQueryNode": {},
	"RetrieverNode": {}, "ApplyRetrieveNode": {}, "OptionalRerankNode": {}, "ProductDocWorkflowNode": {}, "PrepareSerialToolMessageNode": {},
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
	flow := callbackFlow(ctx, input)
	startedAt := time.Now()
	ctx, span := orchestratorobserve.StartSpan(ctx, b.Tracer, info.Name)

	orchestratorobserve.SendEvent(ctx, callbackStreamWriter(flow), "node", map[string]any{"node": info.Name, "status": "start"})
	return context.WithValue(ctx, stateKey{}, callbackState{node: info.Name, startedAt: startedAt, flow: flow, span: span})
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
	flow := st.flow
	if flow == nil {
		flow = callbackFlow(ctx, output)
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
	orchestratorobserve.AppendTraceStep(flow, node, status, latency, detail)
	if b.Metrics != nil {
		b.Metrics.ObserveNode(node, status, latency)
	}
	orchestratorobserve.SendEvent(ctx, callbackStreamWriter(flow), "node", map[string]any{"node": node, "status": status, "latency_ms": latency.Milliseconds(), "detail": detail})
}

func isWorkflowNode(info *callbacks.RunInfo) bool {
	if info == nil {
		return false
	}
	_, ok := nodeNameSet[info.Name]
	return ok
}

func callbackFlow(ctx context.Context, value any) *orchestratorstate.FlowContext {
	if flow, _ := value.(*orchestratorstate.FlowContext); flow != nil {
		return flow
	}
	if payload, _ := value.(map[string]any); payload != nil {
		if flow, _ := payload["flow"].(*orchestratorstate.FlowContext); flow != nil {
			return flow
		}
	}
	var flow *orchestratorstate.FlowContext
	_ = compose.ProcessState[*orchestratorstate.ConversationState](ctx, func(_ context.Context, state *orchestratorstate.ConversationState) error {
		if state != nil {
			flow = state.Flow
		}
		return nil
	})
	return flow
}

func callbackStreamWriter(flow *orchestratorstate.FlowContext) orchestratorstate.StreamWriter {
	if flow == nil {
		return nil
	}
	return flow.StreamWriter
}
