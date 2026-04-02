//go:build legacy_agent

package usecase

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PipelineMetrics Pipeline 鍙娴嬫€ф寚鏍?
// 鎻愪緵 Prometheus 鍩嬬偣锛岃鐩栵細鍚勯樁娈佃€楁椂銆佺紦瀛樺懡涓巼銆佹剰鍥惧垎甯冦€侀檺棰?鐔旀柇/闄嶇骇/杞汉宸ヨ鏁?
type PipelineMetrics struct {
	// ---- 鑰楁椂 ----
	stageDuration *prometheus.HistogramVec // labels: stage (cache|intent|embed|vector|rerank|generate|total)

	// ---- 璁℃暟鍣?----
	cacheHit         prometheus.Counter     // 璇箟缂撳瓨鍛戒腑
	cacheMiss        prometheus.Counter     // 璇箟缂撳瓨鏈懡涓?
	rateLimited      prometheus.Counter     // 闄愰瑙﹀彂
	intentTotal      *prometheus.CounterVec // labels: intent
	llmErrors        prometheus.Counter     // LLM 璋冪敤澶辫触锛堝惈鐔旀柇璺宠繃锛?
	autoEscalations  prometheus.Counter     // 鑷姩杞汉宸ヨЕ鍙?
	emotionTotal     *prometheus.CounterVec // labels: emotion (neutral|mild_frustration|angry|urgent)
	fallbackSync     prometheus.Counter     // 娴佸紡闄嶇骇涓哄悓姝?
	templateFallback prometheus.Counter     // 妯℃澘鍏滃簳瑙﹀彂

	// ---- Gauge ----
	activeSessions prometheus.Gauge // 褰撳墠娲昏穬浼氳瘽鏁帮紙璋冨ぇ/璋冨皬鐢?handler 灞傜鐞嗭級
}

// NewPipelineMetrics 娉ㄥ唽 Prometheus 鎸囨爣
func NewPipelineMetrics() *PipelineMetrics {
	const ns = "douyinmall"
	const sub = "agent"

	return &PipelineMetrics{
		stageDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "pipeline_stage_duration_ms",
			Help:      "Pipeline 鍚勯樁娈佃€楁椂锛堟绉掞級",
			Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000},
		}, []string{"stage"}),

		cacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_hit_total",
			Help: "璇箟缂撳瓨鍛戒腑娆℃暟",
		}),
		cacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_miss_total",
			Help: "璇箟缂撳瓨鏈懡涓鏁?,
		}),
		rateLimited: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "rate_limited_total",
			Help: "闄愰瑙﹀彂娆℃暟",
		}),
		intentTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "intent_total",
			Help: "鍚勬剰鍥捐瘑鍒鏁?,
		}, []string{"intent"}),
		llmErrors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "llm_error_total",
			Help: "LLM 璋冪敤澶辫触鎬绘鏁?,
		}),
		autoEscalations: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "auto_escalation_total",
			Help: "鑷姩杞汉宸ヨЕ鍙戞鏁帮紙鍚綆缃俊搴﹀拰鎯呯华瑙﹀彂锛?,
		}),
		emotionTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "emotion_escalation_total",
			Help: "鍥犵敤鎴锋儏缁Е鍙戣嚜鍔ㄨ浆浜哄伐璁℃暟",
		}, []string{"emotion"}),
		fallbackSync: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "fallback_sync_total",
			Help: "娴佸紡闄嶇骇涓哄悓姝ョ敓鎴愭鏁?,
		}),
		templateFallback: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "template_fallback_total",
			Help: "妯℃澘鍏滃簳瑙﹀彂娆℃暟",
		}),
		activeSessions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub,
			Name: "active_sessions",
			Help: "褰撳墠娲昏穬浼氳瘽鏁?,
		}),
	}
}

// ---- 渚挎嵎鏂规硶 ----

// ObserveStage 璁板綍闃舵鑰楁椂
func (m *PipelineMetrics) ObserveStage(stage string, d time.Duration) {
	m.stageDuration.WithLabelValues(stage).Observe(float64(d.Milliseconds()))
}

// IncCacheHit 缂撳瓨鍛戒腑 +1
func (m *PipelineMetrics) IncCacheHit() { m.cacheHit.Inc() }

// IncCacheMiss 缂撳瓨鏈懡涓?+1
func (m *PipelineMetrics) IncCacheMiss() { m.cacheMiss.Inc() }

// IncRateLimited 闄愰 +1
func (m *PipelineMetrics) IncRateLimited() { m.rateLimited.Inc() }

// IncIntent 鎰忓浘 +1
func (m *PipelineMetrics) IncIntent(intent string) { m.intentTotal.WithLabelValues(intent).Inc() }

// IncLLMError LLM 閿欒 +1
func (m *PipelineMetrics) IncLLMError() { m.llmErrors.Inc() }

// IncAutoEscalation 鑷姩杞汉宸?+1
func (m *PipelineMetrics) IncAutoEscalation() { m.autoEscalations.Inc() }

// IncEmotion 鎯呯华瑙﹀彂鍗囩骇 +1
func (m *PipelineMetrics) IncEmotion(emotion string) {
	m.emotionTotal.WithLabelValues(emotion).Inc()
}

// IncFallbackSync 娴佸紡闄嶇骇鍚屾 +1
func (m *PipelineMetrics) IncFallbackSync() { m.fallbackSync.Inc() }

// IncTemplateFallback 妯℃澘鍏滃簳 +1
func (m *PipelineMetrics) IncTemplateFallback() { m.templateFallback.Inc() }

// IncActiveSessions 娲昏穬浼氳瘽 +1锛堝垱寤轰細璇濇椂璋冪敤锛?
func (m *PipelineMetrics) IncActiveSessions() { m.activeSessions.Inc() }

// DecActiveSessions 娲昏穬浼氳瘽 -1锛堜細璇濆叧闂?杞汉宸ユ椂璋冪敤锛?
func (m *PipelineMetrics) DecActiveSessions() { m.activeSessions.Dec() }
