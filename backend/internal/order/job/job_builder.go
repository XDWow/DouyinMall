package job

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"strconv"
	"time"
)

type CronJobBuilder struct {
	l      logger.LoggerV1
	p      *prometheus.SummaryVec
	tracer trace.Tracer
}

func NewCronJobBuilder(l logger.LoggerV1) *CronJobBuilder {
	p := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "douyin_mall",
		Subsystem: "order",
		Help:      "缁熻瀹氭椂浠诲姟鐨勬墽琛屾儏鍐?,
		Name:      "cron_job",
	}, []string{"name", "success"})
	prometheus.MustRegister(p)
	return &CronJobBuilder{
		l:      l,
		p:      p,
		tracer: otel.GetTracerProvider().Tracer("webook/internal/job"),
	}
}

func (b *CronJobBuilder) Build(job Job) cron.Job {
	name := job.Name()
	return cronJobFuncAdapter(func() error {
		_, span := b.tracer.Start(context.Background(), name)
		defer span.End()
		start := time.Now()
		b.l.Info("浠诲姟寮€濮?,
			logger.String("job", name))
		var success bool
		defer func() {
			b.l.Info("浠诲姟缁撴潫",
				logger.String("job", name))
			duration := time.Since(start).Milliseconds()
			b.p.WithLabelValues(name, strconv.FormatBool(success)).Observe(float64(duration))
		}()
		err := job.Run()
		success = err == nil
		if err != nil {
			span.RecordError(err)
			b.l.Error("杩愯浠诲姟澶辫触", logger.Error(err),
				logger.String("job", name))
		}
		return nil
	})
}

type cronJobFuncAdapter func() error

func (c cronJobFuncAdapter) Run() {
	_ = c()
}


