package observe

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type Metrics struct {
	requestTotal          *prometheus.CounterVec
	requestLatency        *prometheus.SummaryVec
	nodeLatency           *prometheus.SummaryVec
	handoffTotal          *prometheus.CounterVec
	slowestStepLatency    prometheus.Summary
	requestBottleneckNode *prometheus.CounterVec
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
		slowestStepLatency: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  namespace,
			Subsystem:  "chat",
			Name:       "slowest_step_latency_ms",
			Help:       "Per request: latency in ms of the slowest callback-instrumented workflow node (bottleneck severity).",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}),
		requestBottleneckNode: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "chat",
			Name:      "request_bottleneck_node_total",
			Help:      "Per completed request: increments once for whichever workflow node had the highest latency (attribution for optimization).",
		}, []string{"node"}),
	}

	registerCollector(m.requestTotal)
	registerCollector(m.requestLatency)
	registerCollector(m.nodeLatency)
	registerCollector(m.handoffTotal)
	registerCollector(m.slowestStepLatency)
	registerCollector(m.requestBottleneckNode)
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

// ObserveRequestBottleneck 在单次请求收尾时调用：记录最慢节点耗时分布，以及「本请求瓶颈落在哪个节点」的计数。
func (m *Metrics) ObserveRequestBottleneck(slowestNode string, slowestMs int64) {
	if m == nil || slowestNode == "" {
		return
	}
	m.slowestStepLatency.Observe(float64(slowestMs))
	m.requestBottleneckNode.WithLabelValues(slowestNode).Inc()
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

func AppendTraceStep(state *domain.State, node, status string, d time.Duration, detail string) {
	if state == nil {
		return
	}
	resp := state.EnsureResponse()
	resp.Trace.Steps = append(resp.Trace.Steps, domain.TraceStep{
		Node:      node,
		Status:    status,
		LatencyMs: d.Milliseconds(),
		Detail:    detail,
	})
}

// EnrichTraceSlowest 根据 Trace.Steps 填写 SlowestStep*，供 API/日志/Prometheus 归因。
func EnrichTraceSlowest(resp *domain.ChatResult) {
	if resp == nil {
		return
	}
	steps := resp.Trace.Steps
	if len(steps) == 0 {
		return
	}
	var maxMs int64
	var maxNode string
	for _, step := range steps {
		if step.LatencyMs > maxMs {
			maxMs = step.LatencyMs
			maxNode = step.Node
		}
	}
	resp.Trace.SlowestStepNode = maxNode
	resp.Trace.SlowestStepLatencyMs = maxMs
}

func SendEvent(ctx context.Context, writer domain.StreamWriter, event string, data any) {
	if writer == nil {
		return
	}
	_ = writer.Send(ctx, domain.StreamEvent{
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
