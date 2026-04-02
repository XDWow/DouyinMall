package observe

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

type Metrics struct {
	requestTotal   *prometheus.CounterVec
	requestLatency *prometheus.SummaryVec
	nodeLatency    *prometheus.SummaryVec
	handoffTotal   *prometheus.CounterVec
}

func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "agent"
	}

	m := &Metrics{
		requestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "chat",
			Name:      "requests_total",
			Help:      "Total number of AI customer service requests.",
		}, []string{"status"}),
		requestLatency: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: "chat",
			Name:      "request_latency_ms",
			Help:      "Latency of AI customer service requests in milliseconds.",
		}, []string{"status"}),
		nodeLatency: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: "workflow",
			Name:      "node_latency_ms",
			Help:      "Latency of workflow nodes in milliseconds.",
		}, []string{"node", "status"}),
		handoffTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "chat",
			Name:      "handoff_total",
			Help:      "Total number of handoff responses.",
		}, []string{"reason"}),
	}

	registerCollector(m.requestTotal)
	registerCollector(m.requestLatency)
	registerCollector(m.nodeLatency)
	registerCollector(m.handoffTotal)
	return m
}

func (m *Metrics) ObserveRequest(status string, d time.Duration) {
	if m == nil {
		return
	}
	m.requestTotal.WithLabelValues(status).Inc()
	m.requestLatency.WithLabelValues(status).Observe(float64(d.Milliseconds()))
}

func (m *Metrics) ObserveNode(node, status string, d time.Duration) {
	if m == nil {
		return
	}
	m.nodeLatency.WithLabelValues(node, status).Observe(float64(d.Milliseconds()))
}

func (m *Metrics) ObserveHandoff(reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	m.handoffTotal.WithLabelValues(reason).Inc()
}

func registerCollector(collector prometheus.Collector) {
	if collector == nil {
		return
	}
	if err := prometheus.Register(collector); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			panic(err)
		}
	}
}

func AppendTraceStep(flow *orchestratorstate.FlowContext, node, status string, d time.Duration, detail string) {
	if flow == nil {
		return
	}
	resp := flow.EnsureResponse()
	resp.Trace.Steps = append(resp.Trace.Steps, dto.TraceStep{
		Node:      node,
		Status:    status,
		LatencyMs: d.Milliseconds(),
		Detail:    detail,
	})
}

func SendEvent(ctx context.Context, writer orchestratorstate.StreamWriter, event string, data any) {
	if writer == nil {
		return
	}
	_ = writer.Send(ctx, dto.StreamEvent{
		Event: event,
		Data:  data,
	})
}

func StartSpan(ctx context.Context, tracer trace.Tracer, name string) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, name)
}
