//go:build legacy_agent

package usecase

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PipelineMetrics Pipeline 閸欘垵顫囧ù瀣偓褎瀵氶弽?
// 閹绘劒绶?Prometheus 閸╁鍋ｉ敍宀冾洬閻╂牭绱伴崥鍕▉濞堜絻鈧妞傞妴浣虹处鐎涙ê鎳℃稉顓犲芳閵嗕焦鍓伴崶鎯у瀻鐢啨鈧線妾烘０?閻旀梹鏌?闂勫秶楠?鏉烆兛姹夊銉吀閺?
type PipelineMetrics struct {
	// ---- 閼版妞?----
	stageDuration *prometheus.HistogramVec // labels: stage (cache|intent|embed|vector|rerank|generate|total)

	// ---- 鐠佲剝鏆熼崳?----
	cacheHit         prometheus.Counter     // 鐠囶厺绠熺紓鎾崇摠閸涙垝鑵?
	cacheMiss        prometheus.Counter     // 鐠囶厺绠熺紓鎾崇摠閺堫亜鎳℃稉?
	rateLimited      prometheus.Counter     // 闂勬劙顣剁憴锕€褰?
	intentTotal      *prometheus.CounterVec // labels: intent
	llmErrors        prometheus.Counter     // LLM 鐠嬪啰鏁ゆ径杈Е閿涘牆鎯堥悢鏃€鏌囩捄瀹犵箖閿?
	autoEscalations  prometheus.Counter     // 閼奉亜濮╂潪顑挎眽瀹搞儴袝閸?
	emotionTotal     *prometheus.CounterVec // labels: emotion (neutral|mild_frustration|angry|urgent)
	fallbackSync     prometheus.Counter     // 濞翠礁绱￠梽宥囬獓娑撳搫鎮撳?
	templateFallback prometheus.Counter     // 濡剝婢橀崗婊冪俺鐟欙箑褰?

	// ---- Gauge ----
	activeSessions prometheus.Gauge // 瑜版挸澧犲ú鏄忕┈娴兼俺鐦介弫甯礄鐠嬪啫銇?鐠嬪啫鐨悽?handler 鐏炲倻顓搁悶鍡礆
}

// NewPipelineMetrics 濞夈劌鍞?Prometheus 閹稿洦鐖?
func NewPipelineMetrics() *PipelineMetrics {
	const ns = "douyinmall"
	const sub = "agent"

	return &PipelineMetrics{
		stageDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "pipeline_stage_duration_ms",
			Help:      "Pipeline 閸氬嫰妯佸▓浣冣偓妤佹閿涘牊顕犵粔鎺炵礆",
			Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000},
		}, []string{"stage"}),

		cacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_hit_total",
			Help: "鐠囶厺绠熺紓鎾崇摠閸涙垝鑵戝▎鈩冩殶",
		}),
		cacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "cache_miss_total",
			Help: "鐠囶厺绠熺紓鎾崇摠閺堫亜鎳℃稉顓燁偧閺?,
		}),
		rateLimited: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "rate_limited_total",
			Help: "闂勬劙顣剁憴锕€褰傚▎鈩冩殶",
		}),
		intentTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "intent_total",
			Help: "閸氬嫭鍓伴崶鎹愮槕閸掝偉顓搁弫?,
		}, []string{"intent"}),
		llmErrors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "llm_error_total",
			Help: "LLM 鐠嬪啰鏁ゆ径杈Е閹粯顐奸弫?,
		}),
		autoEscalations: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "auto_escalation_total",
			Help: "閼奉亜濮╂潪顑挎眽瀹搞儴袝閸欐垶顐奸弫甯礄閸氼偂缍嗙純顔讳繆鎼达箑鎷伴幆鍛崕鐟欙箑褰傞敍?,
		}),
		emotionTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "emotion_escalation_total",
			Help: "閸ョ姷鏁ら幋閿嬪剰缂侇亣袝閸欐垼鍤滈崝銊ㄦ祮娴滃搫浼愮拋鈩冩殶",
		}, []string{"emotion"}),
		fallbackSync: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "fallback_sync_total",
			Help: "濞翠礁绱￠梽宥囬獓娑撳搫鎮撳銉ф晸閹存劖顐奸弫?,
		}),
		templateFallback: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub,
			Name: "template_fallback_total",
			Help: "濡剝婢橀崗婊冪俺鐟欙箑褰傚▎鈩冩殶",
		}),
		activeSessions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub,
			Name: "active_sessions",
			Help: "瑜版挸澧犲ú鏄忕┈娴兼俺鐦介弫?,
		}),
	}
}

// ---- 娓氭寧宓庨弬瑙勭《 ----

// ObserveStage 鐠佹澘缍嶉梼鑸殿唽閼版妞?
func (m *PipelineMetrics) ObserveStage(stage string, d time.Duration) {
	m.stageDuration.WithLabelValues(stage).Observe(float64(d.Milliseconds()))
}

// IncCacheHit 缂傛挸鐡ㄩ崨鎴掕厬 +1
func (m *PipelineMetrics) IncCacheHit() { m.cacheHit.Inc() }

// IncCacheMiss 缂傛挸鐡ㄩ張顏勬嚒娑?+1
func (m *PipelineMetrics) IncCacheMiss() { m.cacheMiss.Inc() }

// IncRateLimited 闂勬劙顣?+1
func (m *PipelineMetrics) IncRateLimited() { m.rateLimited.Inc() }

// IncIntent 閹板繐娴?+1
func (m *PipelineMetrics) IncIntent(intent string) { m.intentTotal.WithLabelValues(intent).Inc() }

// IncLLMError LLM 闁挎瑨顕?+1
func (m *PipelineMetrics) IncLLMError() { m.llmErrors.Inc() }

// IncAutoEscalation 閼奉亜濮╂潪顑挎眽瀹?+1
func (m *PipelineMetrics) IncAutoEscalation() { m.autoEscalations.Inc() }

// IncEmotion 閹懐鍗庣憴锕€褰傞崡鍥╅獓 +1
func (m *PipelineMetrics) IncEmotion(emotion string) {
	m.emotionTotal.WithLabelValues(emotion).Inc()
}

// IncFallbackSync 濞翠礁绱￠梽宥囬獓閸氬本顒?+1
func (m *PipelineMetrics) IncFallbackSync() { m.fallbackSync.Inc() }

// IncTemplateFallback 濡剝婢橀崗婊冪俺 +1
func (m *PipelineMetrics) IncTemplateFallback() { m.templateFallback.Inc() }

// IncActiveSessions 濞叉槒绌导姘崇樈 +1閿涘牆鍨卞杞扮窗鐠囨繃妞傜拫鍐暏閿?
func (m *PipelineMetrics) IncActiveSessions() { m.activeSessions.Inc() }

// DecActiveSessions 濞叉槒绌导姘崇樈 -1閿涘牅绱扮拠婵嗗彠闂?鏉烆兛姹夊銉︽鐠嬪啰鏁ら敍?
func (m *PipelineMetrics) DecActiveSessions() { m.activeSessions.Dec() }


