package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/cloudwego/kitex/client"
)

type config struct {
	seckillAddr      string
	runID            string
	activityCount    int
	hotActivityCount int
	hotPercent       int
	totalRequests    int
	stockFactor      float64
	windowSeconds    int
}

type output struct {
	RunID                string  `json:"run_id"`
	ActivityCount        int     `json:"activity_count"`
	HotActivityCount     int     `json:"hot_activity_count"`
	HotPercent           int     `json:"hot_percent"`
	TotalRequests        int     `json:"total_requests"`
	StockFactor          float64 `json:"stock_factor"`
	TotalStock           int     `json:"total_stock"`
	HotPerActivityStock  int     `json:"hot_per_activity_stock"`
	ColdPerActivityStock int     `json:"cold_per_activity_stock"`
	HotActivityIDs       []int64 `json:"hot_activity_ids"`
	ColdActivityIDs      []int64 `json:"cold_activity_ids"`
}

func main() {
	cfg := parseFlags()

	client, err := seckillservice.NewClient("seckill.service", client.WithHostPorts(cfg.seckillAddr))
	if err != nil {
		fatalf("create seckill client: %v", err)
	}

	totalStock := int(math.Ceil(float64(cfg.totalRequests) * cfg.stockFactor))
	if totalStock < cfg.totalRequests {
		totalStock = cfg.totalRequests
	}
	hotTotal := int(math.Ceil(float64(totalStock) * float64(cfg.hotPercent) / 100))
	coldTotal := totalStock - hotTotal
	coldActivityCount := max(cfg.activityCount-cfg.hotActivityCount, 0)

	hotPerActivity := max(int(math.Ceil(float64(hotTotal)/float64(cfg.hotActivityCount))), 1)
	coldPerActivity := 0
	if coldActivityCount > 0 {
		coldPerActivity = max(int(math.Ceil(float64(coldTotal)/float64(coldActivityCount))), 1)
	}

	now := time.Now()
	hotIDs := make([]int64, 0, cfg.hotActivityCount)
	coldIDs := make([]int64, 0, coldActivityCount)
	for i := 0; i < cfg.activityCount; i++ {
		isHot := i < cfg.hotActivityCount
		stock := coldPerActivity
		price := int64(109)
		if isHot {
			stock = hotPerActivity
			price = 99
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.CreateActivity(ctx, &seckillv1.CreateActivityReq{
			ActivityName: fmt.Sprintf("perf-seckill-%s-%d", cfg.runID, i),
			ProductId:    now.UnixNano()%1_000_000 + 3_000_000 + int64(i),
			SkuId:        now.UnixNano()%1_000_000 + 4_000_000 + int64(i),
			SeckillPrice: price,
			TotalStock:   int32(stock),
			StartTime:    now.Add(-time.Minute).Unix(),
			EndTime:      now.Add(time.Duration(cfg.windowSeconds) * time.Second).Unix(),
			Status:       "ONLINE",
			LimitPerUser: 1,
		})
		cancel()
		if err != nil {
			fatalf("create activity %d: %v", i, err)
		}
		if isHot {
			hotIDs = append(hotIDs, resp.GetActivityId())
		} else {
			coldIDs = append(coldIDs, resp.GetActivityId())
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output{
		RunID:                cfg.runID,
		ActivityCount:        cfg.activityCount,
		HotActivityCount:     cfg.hotActivityCount,
		HotPercent:           cfg.hotPercent,
		TotalRequests:        cfg.totalRequests,
		StockFactor:          cfg.stockFactor,
		TotalStock:           totalStock,
		HotPerActivityStock:  hotPerActivity,
		ColdPerActivityStock: coldPerActivity,
		HotActivityIDs:       hotIDs,
		ColdActivityIDs:      coldIDs,
	})
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.seckillAddr, "seckill_addr", "127.0.0.1:8098", "seckill service grpc addr")
	flag.StringVar(&cfg.runID, "run_id", fmt.Sprintf("%d", time.Now().UnixNano()), "run id suffix")
	flag.IntVar(&cfg.activityCount, "activity_count", 8, "total activity count")
	flag.IntVar(&cfg.hotActivityCount, "hot_activity_count", 2, "hot activity count")
	flag.IntVar(&cfg.hotPercent, "hot_percent", 80, "hot traffic percent")
	flag.IntVar(&cfg.totalRequests, "total_requests", 60000, "estimated total requests for the test window")
	flag.Float64Var(&cfg.stockFactor, "stock_factor", 1.25, "stock factor applied to total requests")
	flag.IntVar(&cfg.windowSeconds, "window_seconds", 3600, "activity valid window seconds")
	flag.Parse()

	if cfg.activityCount <= 0 {
		fatalf("activity_count must be > 0")
	}
	if cfg.hotActivityCount <= 0 || cfg.hotActivityCount > cfg.activityCount {
		fatalf("hot_activity_count must be within [1, activity_count]")
	}
	if cfg.hotPercent < 0 || cfg.hotPercent > 100 {
		fatalf("hot_percent must be within [0,100]")
	}
	if cfg.totalRequests <= 0 {
		fatalf("total_requests must be > 0")
	}
	if cfg.stockFactor <= 0 {
		fatalf("stock_factor must be > 0")
	}
	return cfg
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
