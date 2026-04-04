package redisx

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"net"
	"strconv"
	"time"
)

type PrometheusHook struct {
	vector *prometheus.SummaryVec
}

func NewPrometheusHook(opt prometheus.SummaryOpts) *PrometheusHook {
	vector := prometheus.NewSummaryVec(opt,
		// key_exist 鏄惁鍛戒腑缂撳瓨
		[]string{"cmd", "key_exist"})
	prometheus.MustRegister(vector)
	return &PrometheusHook{
		vector: vector,
	}
}

func (p *PrometheusHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (p *PrometheusHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		// 鍦≧edis鎵ц涔嬪墠
		startTime := time.Now()
		var err error
		defer func() {
			duration := time.Since(startTime).Milliseconds()
			//biz := ctx.Value("biz")
			keyExist := err != redis.Nil
			p.vector.WithLabelValues(cmd.Name(), strconv.FormatBool(keyExist)).Observe(float64(duration))
		}()
		// 杩欎釜浼氭渶缁堝彂閫佸懡浠ゅ埌 redis 涓?
		err = next(ctx, cmd)
		// redis 鎵ц瀹屼簡
		return err
	}
}

func (p *PrometheusHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	//TODO implement me
	panic("implement me")
}


