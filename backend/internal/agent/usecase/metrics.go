package usecase

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PipelineMetrics Pipeline 可观测性指标
// 提供 Prometheus 埋点，覆盖：各阶段耗时、缓存命中率、意图分布、限频/熔断/降级/转人工计数
type PipelineMetrics struct {
	// ---- 耗时 ----
	stageDuration *prometheus.HistogramVec // labels: stage (cache|intent|embed|vector|rerank|generate|total)

	// ---- 计数器 ----
	cacheHit         prometheus.Counter     // 语义缓存命中
	cacheMiss        prometheus.Counter     // 语义缓存未命中
	rateLimited      prometheus.Counter     // 限频触发
	intentTotal      *prometheus.CounterVec // labels: intent
	llmErrors        prometheus.Counter     // LLM 调用失败（含熔断跳过）
	autoEscalations  prometheus.Counter     // 自动转人工触发
	emotionTotal     *prometheus.CounterVec // labels: emotion (neutral|mild_frustration|angry|urgent)
	fallbackSync     prometheus.Counter     // 流式降级为同步
	templateFallback prometheus.Counter     // 模板兜底触发

	// ---- Gauge ----
	activeSessions prometheus.Gauge // 当前活跃会话数（调大/调小由 handler 层管理）
}

// NewPipelineMetrics 注册 Prometheus 指标
func NewPipelineMetrics() *PipelineMetrics {
	const ns = "douyinmall"
	const sub = "agent"

	return &PipelineMetrics{
		stageDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "pipeline_stage_duration_ms",
			Help:      "Pipeline 各阶段耗时（毫秒）",
			Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000},
		}, []string{"stage"}),

		cacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_hit_total",
			Help: "语义缓存命中次数",
		}),
		cacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_miss_total",
			Help: "语义缓存未命中次数",
		}),
		rateLimited: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "rate_limited_total",
			Help: "限频触发次数",
		}),
		intentTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "intent_total",
			Help: "各意图识别计数",
		}, []string{"intent"}),
		llmErrors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "llm_error_total",
			Help: "LLM 调用失败总次数",
		}),
		autoEscalations: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "auto_escalation_total",
			Help: "自动转人工触发次数（含低置信度和情绪触发）",
		}),
		emotionTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "emotion_escalation_total",
			Help: "因用户情绪触发自动转人工计数",
		}, []string{"emotion"}),
		fallbackSync: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "fallback_sync_total",
			Help: "流式降级为同步生成次数",
		}),
		templateFallback: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "template_fallback_total",
			Help: "模板兜底触发次数",
		}),
		activeSessions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub,
			Name: "active_sessions",
			Help: "当前活跃会话数",
		}),
	}
}

// ---- 便捷方法 ----

// ObserveStage 记录阶段耗时
func (m *PipelineMetrics) ObserveStage(stage string, d time.Duration) {
	m.stageDuration.WithLabelValues(stage).Observe(float64(d.Milliseconds()))
}

// IncCacheHit 缓存命中 +1
func (m *PipelineMetrics) IncCacheHit() { m.cacheHit.Inc() }

// IncCacheMiss 缓存未命中 +1
func (m *PipelineMetrics) IncCacheMiss() { m.cacheMiss.Inc() }

// IncRateLimited 限频 +1
func (m *PipelineMetrics) IncRateLimited() { m.rateLimited.Inc() }

// IncIntent 意图 +1
func (m *PipelineMetrics) IncIntent(intent string) { m.intentTotal.WithLabelValues(intent).Inc() }

// IncLLMError LLM 错误 +1
func (m *PipelineMetrics) IncLLMError() { m.llmErrors.Inc() }

// IncAutoEscalation 自动转人工 +1
func (m *PipelineMetrics) IncAutoEscalation() { m.autoEscalations.Inc() }

// IncEmotion 情绪触发升级 +1
func (m *PipelineMetrics) IncEmotion(emotion string) {
	m.emotionTotal.WithLabelValues(emotion).Inc()
}

// IncFallbackSync 流式降级同步 +1
func (m *PipelineMetrics) IncFallbackSync() { m.fallbackSync.Inc() }

// IncTemplateFallback 模板兜底 +1
func (m *PipelineMetrics) IncTemplateFallback() { m.templateFallback.Inc() }

// IncActiveSessions 活跃会话 +1（创建会话时调用）
func (m *PipelineMetrics) IncActiveSessions() { m.activeSessions.Inc() }

// DecActiveSessions 活跃会话 -1（会话关闭/转人工时调用）
func (m *PipelineMetrics) DecActiveSessions() { m.activeSessions.Dec() }
