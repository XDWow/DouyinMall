package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/compose"
	"go.opentelemetry.io/otel/codes"
)

func (s *Service) buildGraph(ctx context.Context) (compose.Runnable[*FlowContext, *FlowContext], error) {
	g := compose.NewGraph[*FlowContext, *FlowContext]()

	for _, item := range []struct {
		name string
		fn   func(context.Context, *FlowContext) (*FlowContext, error)
	}{
		{name: "SessionGuardNode", fn: s.sessionGuardNode},
		{name: "CacheLookupNode", fn: s.cacheLookupNode},
		{name: "IntentClassifyNode", fn: s.intentClassifyNode},
		{name: "RiskGuardNode", fn: s.riskGuardNode},
		{name: "RewriteNode", fn: s.rewriteNode},
		{name: "RetrieveNode", fn: s.retrieveNode},
		{name: "RerankNode", fn: s.rerankNode},
		{name: "ToolDecisionNode", fn: s.toolDecisionNode},
		{name: "ToolExecNode", fn: s.toolExecNode},
		{name: "AnswerComposeNode", fn: s.answerComposeNode},
		{name: "ConfidenceGuardNode", fn: s.confidenceGuardNode},
		{name: "HandoffNode", fn: s.handoffNode},
		{name: "PersistNode", fn: s.persistNode},
	} {
		if err := g.AddLambdaNode(item.name, s.wrapNode(item.name, item.fn), compose.WithNodeName(item.name)); err != nil {
			return nil, err
		}
	}

	mustAddEdge := func(start, end string) error {
		if err := g.AddEdge(start, end); err != nil {
			return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
		}
		return nil
	}

	if err := mustAddEdge(compose.START, "SessionGuardNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("SessionGuardNode", "CacheLookupNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("IntentClassifyNode", "RiskGuardNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("RewriteNode", "RetrieveNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("RetrieveNode", "RerankNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("RerankNode", "ToolDecisionNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("ToolExecNode", "AnswerComposeNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("AnswerComposeNode", "ConfidenceGuardNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("HandoffNode", "PersistNode"); err != nil {
		return nil, err
	}
	if err := mustAddEdge("PersistNode", compose.END); err != nil {
		return nil, err
	}

	if err := g.AddBranch("CacheLookupNode", compose.NewGraphBranch(
		func(ctx context.Context, in *FlowContext) (string, error) {
			if in != nil && in.CacheItem != nil {
				return "PersistNode", nil
			}
			return "IntentClassifyNode", nil
		},
		map[string]bool{"PersistNode": true, "IntentClassifyNode": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("RiskGuardNode", compose.NewGraphBranch(
		func(ctx context.Context, in *FlowContext) (string, error) {
			if in != nil && in.Risk.ForceHandoff {
				return "HandoffNode", nil
			}
			return "RewriteNode", nil
		},
		map[string]bool{"HandoffNode": true, "RewriteNode": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("ToolDecisionNode", compose.NewGraphBranch(
		func(ctx context.Context, in *FlowContext) (string, error) {
			if in != nil && in.Tool.NeedTool && len(in.Tool.Plans) > 0 {
				return "ToolExecNode", nil
			}
			return "AnswerComposeNode", nil
		},
		map[string]bool{"ToolExecNode": true, "AnswerComposeNode": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("ConfidenceGuardNode", compose.NewGraphBranch(
		func(ctx context.Context, in *FlowContext) (string, error) {
			if in != nil && in.ensureResponse().NeedHandoff {
				return "HandoffNode", nil
			}
			return "PersistNode", nil
		},
		map[string]bool{"HandoffNode": true, "PersistNode": true},
	)); err != nil {
		return nil, err
	}

	opts := []compose.GraphCompileOption{
		compose.WithGraphName("agent_customer_service"),
		compose.WithMaxRunSteps(32),
	}
	if s.checkpointStore != nil {
		opts = append(opts, compose.WithCheckPointStore(s.checkpointStore))
	}
	if len(s.cfg.InterruptBeforeNodes) > 0 {
		opts = append(opts, compose.WithInterruptBeforeNodes(s.cfg.InterruptBeforeNodes))
	}
	if len(s.cfg.InterruptAfterNodes) > 0 {
		opts = append(opts, compose.WithInterruptAfterNodes(s.cfg.InterruptAfterNodes))
	}

	return g.Compile(ctx, opts...)
}

func (s *Service) wrapNode(name string, fn func(context.Context, *FlowContext) (*FlowContext, error)) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, in *FlowContext) (*FlowContext, error) {
		start := time.Now()
		ctx, span := startSpan(ctx, s.tracer, name)
		defer span.End()

		sendEvent(ctx, in.StreamWriter, "node", map[string]any{
			"node":   name,
			"status": "start",
		})

		out, err := fn(ctx, in)
		if out == nil {
			out = in
		}

		status := "ok"
		detail := ""
		if err != nil {
			status = "error"
			detail = err.Error()
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "ok")
		}

		latency := time.Since(start)
		appendTraceStep(out, name, status, latency, detail)
		s.metrics.ObserveNode(name, status, latency)

		sendEvent(ctx, out.StreamWriter, "node", map[string]any{
			"node":       name,
			"status":     status,
			"latency_ms": latency.Milliseconds(),
			"detail":     detail,
		})

		return out, err
	})
}
