package metric

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

type MiddlewareBuilder struct {
	Namespace  string
	Subsystem  string
	Name       string
	Help       string
	InstanceID string
}

func NewBuilder(Namespace string, Subsystem string,
	Name string, Help string, InstanceID string) *MiddlewareBuilder {
	return &MiddlewareBuilder{
		Namespace:  Namespace,
		Subsystem:  Subsystem,
		Name:       Name,
		Help:       Help,
		InstanceID: InstanceID,
	}
}

func (m *MiddlewareBuilder) Build() gin.HandlerFunc {
	// pattern 是指你命中的路由
	// 是指你的 HTTP 的 status
	// path /detail/1
	labels := []string{"method", "pattern", "status"}
	summary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: m.Namespace,
		Subsystem: m.Subsystem,
		Name:      m.Name + "_resp_time",
		Help:      m.Help,
		ConstLabels: prometheus.Labels{
			"instance_id": m.InstanceID,
		},
		Objectives: map[float64]float64{
			0.5:   0.01,
			0.9:   0.01,
			0.99:  0.005,
			0.999: 0.0001,
		},
	}, labels)
	prometheus.MustRegister(summary)

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: m.Namespace,
		Subsystem: m.Subsystem,
		Name:      m.Name + "_active_req",
		Help:      m.Help,
		ConstLabels: map[string]string{
			"instance_id": m.InstanceID,
		},
	})
	prometheus.MustRegister(gauge)

	return func(ctx *gin.Context) {
		startTime := time.Now()
		gauge.Inc()
		defer func() {
			duration := time.Since(startTime)
			gauge.Dec()
			// 告诉大家，也许404???? 而不是我系统错误
			pattern := ctx.FullPath()
			if pattern == "" {
				pattern = "unknown"
			}
			summary.WithLabelValues(ctx.Request.Method, pattern, strconv.Itoa(ctx.Writer.Status())).
				Observe(float64(duration.Milliseconds()))
		}()
		ctx.Next()
	}
}
