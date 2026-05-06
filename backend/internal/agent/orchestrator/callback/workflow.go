package callback

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
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
	state     *domain.State
	span      trace.Span
}

var nodeNameSet = map[string]struct{}{
	"AccessGuardNode":                 {},
	"SessionLoadNode":                 {},
	"UnderstandingNode":               {},
	"RouteNode":                       {},
	"ProductServiceGraph":             {},
	"ProductServiceAgentNode":         {},
	"OrderServiceGraph":               {},
	"OrderServiceAgentNode":           {},
	"PromotionServiceGraph":           {},
	"PromotionCacheLookupNode":        {},
	"PromotionCacheResultNode":        {},
	"PromotionRAGNode":                {},
	"PromotionServiceAgentNode":       {},
	"AftersalesPolicyGraph":           {},
	"AftersalesPolicyCacheLookupNode": {},
	"AftersalesPolicyCacheResultNode": {},
	"AftersalesPolicyRAGNode":         {},
	"AftersalesPolicyAgentNode":       {},
	"AddToCartGraph":                  {},
	"AddToCartResolveNode":            {},
	"AddToCartEnsureArgsNode":         {},
	"AddToCartSubmitNode":             {},
	"AftersalesApplyGraph":            {},
	"AftersalesApplyResolveNode":      {},
	"AftersalesApplyEnsureArgsNode":   {},
	"AftersalesApplyConfirmNode":      {},
	"AftersalesApplySubmitNode":       {},
	"UnknownGraph":                    {},
	"UnknownNode":                     {},
	"FinalizeNode":                    {},
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

	orchestratorobserve.SendEvent(ctx, agenttool.StreamWriterFrom(ctx), "node", map[string]any{"node": info.Name, "status": "start"})
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
	orchestratorobserve.SendEvent(ctx, agenttool.StreamWriterFrom(ctx), "node", map[string]any{"node": node, "status": status, "latency_ms": latency.Milliseconds(), "detail": detail})
}

func isWorkflowNode(info *callbacks.RunInfo) bool {
	if info == nil {
		return false
	}
	_, ok := nodeNameSet[info.Name]
	return ok
}

func callbackFlow(ctx context.Context, value any) *domain.State {
	if state, _ := value.(*domain.State); state != nil {
		return state
	}
	if payload, _ := value.(map[string]any); payload != nil {
		if state, _ := payload["state"].(*domain.State); state != nil {
			return state
		}
	}
	var state *domain.State
	_ = compose.ProcessState[*domain.State](ctx, func(_ context.Context, current *domain.State) error {
		if current != nil {
			state = current
		}
		return nil
	})
	return state
}
