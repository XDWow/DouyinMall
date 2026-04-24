package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	checkoutv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/cloudwego/kitex/client"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type benchConfig struct {
	mode          string
	checkoutAddr  string
	productAddr   string
	inventoryAddr string
	orderAddr     string
	seckillAddr   string
	mysqlDSN      string
	redisAddr     string
	mockWechatURL string
	requests      int
	concurrency   int
	duplicateFan  int
}

type metricSummary struct {
	Scenario          string  `json:"scenario"`
	Requests          int     `json:"requests"`
	Concurrency       int     `json:"concurrency"`
	Success           int64   `json:"success"`
	Failure           int64   `json:"failure"`
	SuccessRate       float64 `json:"success_rate"`
	RPS               float64 `json:"rps"`
	P50Millis         float64 `json:"p50_ms"`
	P90Millis         float64 `json:"p90_ms"`
	P99Millis         float64 `json:"p99_ms"`
	TotalDurationMS   float64 `json:"total_duration_ms"`
	ExtraDescription  string  `json:"extra_description,omitempty"`
	CorrectedRate     float64 `json:"corrected_rate"`
	MisclosedOrders   int     `json:"misclosed_orders"`
	OversoldCount     int     `json:"oversold_count"`
	DuplicateUsers    int     `json:"duplicate_users"`
	DuplicateRate     float64 `json:"duplicate_rate"`
	ExpectedStock     int     `json:"expected_stock"`
	FinalSuccessCount int     `json:"final_success_count"`
}

func main() {
	cfg := parseFlags()

	var (
		summary metricSummary
		err     error
	)

	switch cfg.mode {
	case "checkout":
		summary, err = runCheckoutBench(cfg)
	case "payment_consistency":
		summary, err = runPaymentConsistency(cfg)
	case "seckill":
		summary, err = runSeckillBench(cfg)
	case "seckill_duplicate":
		summary, err = runSeckillDuplicate(cfg)
	default:
		err = fmt.Errorf("unsupported mode: %s", cfg.mode)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "bench failed: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(summary)
}

func parseFlags() benchConfig {
	cfg := benchConfig{}
	flag.StringVar(&cfg.mode, "mode", "", "checkout|payment_consistency|seckill|seckill_duplicate")
	flag.StringVar(&cfg.checkoutAddr, "checkout_addr", "127.0.0.1:8086", "checkout service addr")
	flag.StringVar(&cfg.productAddr, "product_addr", "127.0.0.1:8096", "product service addr")
	flag.StringVar(&cfg.inventoryAddr, "inventory_addr", "127.0.0.1:8094", "inventory service addr")
	flag.StringVar(&cfg.orderAddr, "order_addr", "127.0.0.1:8095", "order service addr")
	flag.StringVar(&cfg.seckillAddr, "seckill_addr", "127.0.0.1:8098", "seckill service addr")
	flag.StringVar(&cfg.mysqlDSN, "mysql_dsn", "root:root@tcp(127.0.0.1:13306)/douyinmall?charset=utf8mb4&parseTime=True&loc=Local", "mysql dsn")
	flag.StringVar(&cfg.redisAddr, "redis_addr", "127.0.0.1:16379", "redis addr")
	flag.StringVar(&cfg.mockWechatURL, "mock_wechat_url", "http://127.0.0.1:8888", "mock wechat base url")
	flag.IntVar(&cfg.requests, "requests", 200, "total requests")
	flag.IntVar(&cfg.concurrency, "concurrency", 20, "worker count")
	flag.IntVar(&cfg.duplicateFan, "duplicate_fan", 5, "duplicate requests per user")
	flag.Parse()
	return cfg
}

func runCheckoutBench(cfg benchConfig) (metricSummary, error) {
	checkoutClient, err := checkoutservice.NewClient("checkout.service", client.WithHostPorts(cfg.checkoutAddr))
	if err != nil {
		return metricSummary{}, err
	}
	productClient, err := productservice.NewClient("product.service", client.WithHostPorts(cfg.productAddr))
	if err != nil {
		return metricSummary{}, err
	}
	inventoryClient, err := inventoryservice.NewClient("inventory.service", client.WithHostPorts(cfg.inventoryAddr))
	if err != nil {
		return metricSummary{}, err
	}

	productID, err := seedProductAndStock(productClient, inventoryClient, cfg.requests+20, 299)
	if err != nil {
		return metricSummary{}, err
	}
	skuID := productID + 1_000_000_000

	durations, success, failure, elapsed := runWorkers(cfg.requests, cfg.concurrency, func(i int) error {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		userID := int64(1_000_000 + i)
		_, err := checkoutClient.PlaceOrder(ctx, &checkoutv1.PlaceOrderReq{
			UserId: userID,
			Items: []*checkoutv1.CheckoutItem{{
				ProductId: productID,
				SkuId:     skuID,
				Quantity:  1,
			}},
			Address: &checkoutv1.Address{
				ReceiverName: "bench buyer",
				Phone:        "13800138000",
				Province:     "Shanghai",
				City:         "Shanghai",
				District:     "Pudong",
				Street:       "No.1 Bench Road",
				ZipCode:      "200120",
			},
			PaymentMethod:  "WECHAT_NATIVE",
			Currency:       "CNY",
			ExpectedAmount: 299,
		})
		return err
	})

	return buildSummary("checkout_place_order", cfg.requests, cfg.concurrency, success, failure, durations, elapsed, ""), nil
}

func runPaymentConsistency(cfg benchConfig) (metricSummary, error) {
	checkoutClient, err := checkoutservice.NewClient("checkout.service", client.WithHostPorts(cfg.checkoutAddr))
	if err != nil {
		return metricSummary{}, err
	}
	productClient, err := productservice.NewClient("product.service", client.WithHostPorts(cfg.productAddr))
	if err != nil {
		return metricSummary{}, err
	}
	inventoryClient, err := inventoryservice.NewClient("inventory.service", client.WithHostPorts(cfg.inventoryAddr))
	if err != nil {
		return metricSummary{}, err
	}
	orderClient, err := orderservice.NewClient("order.service", client.WithHostPorts(cfg.orderAddr))
	if err != nil {
		return metricSummary{}, err
	}

	db, err := sql.Open("mysql", cfg.mysqlDSN)
	if err != nil {
		return metricSummary{}, err
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.redisAddr})
	defer func() { _ = rdb.Close() }()

	productID, err := seedProductAndStock(productClient, inventoryClient, cfg.requests+20, 399)
	if err != nil {
		return metricSummary{}, err
	}
	skuID := productID + 2_000_000_000

	orderIDs := make([]int64, 0, cfg.requests)
	for i := 0; i < cfg.requests; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		resp, placeErr := checkoutClient.PlaceOrder(ctx, &checkoutv1.PlaceOrderReq{
			UserId: int64(2_000_000 + i),
			Items: []*checkoutv1.CheckoutItem{{
				ProductId: productID,
				SkuId:     skuID,
				Quantity:  1,
			}},
			Address: &checkoutv1.Address{
				ReceiverName: "consistency buyer",
				Phone:        "13800138001",
				Province:     "Shanghai",
				City:         "Shanghai",
				District:     "Minhang",
				Street:       "No.2 Consistency Road",
				ZipCode:      "200240",
			},
			PaymentMethod:  "WECHAT_NATIVE",
			Currency:       "CNY",
			ExpectedAmount: 399,
		})
		cancel()
		if placeErr != nil {
			return metricSummary{}, placeErr
		}
		orderIDs = append(orderIDs, resp.GetOrderId())
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	for _, orderID := range orderIDs {
		req, reqErr := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/mock/pay/%d?callback=false", cfg.mockWechatURL, orderID), nil)
		if reqErr != nil {
			return metricSummary{}, reqErr
		}
		resp, doErr := httpClient.Do(req)
		if doErr != nil {
			return metricSummary{}, doErr
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return metricSummary{}, fmt.Errorf("mock pay without callback failed for order %d: status=%d", orderID, resp.StatusCode)
		}
	}

	nowMillis := float64(time.Now().Add(-2 * time.Minute).UnixMilli())
	for _, orderID := range orderIDs {
		if _, err = db.ExecContext(context.Background(), "UPDATE orders SET expired_at = DATE_SUB(UTC_TIMESTAMP(), INTERVAL 2 MINUTE) WHERE id = ?", orderID); err != nil {
			return metricSummary{}, err
		}
		if err = rdb.ZAdd(context.Background(), "order:timeout:queue", redis.Z{
			Score:  nowMillis,
			Member: strconv.FormatInt(orderID, 10),
		}).Err(); err != nil {
			return metricSummary{}, err
		}
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		allDone := true
		for _, orderID := range orderIDs {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			orderResp, getErr := orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: orderID})
			cancel()
			if getErr != nil {
				allDone = false
				break
			}
			status := orderResp.GetOrder().GetOrderStatus()
			if status == orderv1.OrderStatus_ORDER_STATUS_CREATED {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		time.Sleep(1 * time.Second)
	}

	paidCount := 0
	misclosedCount := 0
	for _, orderID := range orderIDs {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		orderResp, getErr := orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: orderID})
		cancel()
		if getErr != nil {
			return metricSummary{}, getErr
		}
		switch orderResp.GetOrder().GetOrderStatus() {
		case orderv1.OrderStatus_ORDER_STATUS_PAID:
			paidCount++
		case orderv1.OrderStatus_ORDER_STATUS_CANCELED:
			misclosedCount++
		}
	}

	return metricSummary{
		Scenario:         "payment_callback_missing_consistency",
		Requests:         cfg.requests,
		Concurrency:      1,
		Success:          int64(paidCount),
		Failure:          int64(cfg.requests - paidCount),
		SuccessRate:      float64(paidCount) / float64(cfg.requests),
		CorrectedRate:    float64(paidCount) / float64(cfg.requests),
		MisclosedOrders:  misclosedCount,
		ExtraDescription: "mark third-party payment success without callback, then force timeout close path to rely on payment confirm",
	}, nil
}

func runSeckillBench(cfg benchConfig) (metricSummary, error) {
	seckillClient, err := seckillservice.NewClient("seckill.service", client.WithHostPorts(cfg.seckillAddr))
	if err != nil {
		return metricSummary{}, err
	}
	db, err := sql.Open("mysql", cfg.mysqlDSN)
	if err != nil {
		return metricSummary{}, err
	}
	defer db.Close()

	stock := cfg.requests / 4
	if stock < 50 {
		stock = 50
	}
	activityID, err := createSeckillActivity(seckillClient, stock, 99)
	if err != nil {
		return metricSummary{}, err
	}

	durations, success, failure, elapsed := runWorkers(cfg.requests, cfg.concurrency, func(i int) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := seckillClient.SubmitSeckill(ctx, &seckillv1.SubmitSeckillReq{
			ActivityId: activityID,
			UserId:     int64(3_000_000 + i),
		})
		return err
	})

	waitErr := waitForSeckillSettlement(db, activityID, time.Now().Add(60*time.Second))
	if waitErr != nil {
		return metricSummary{}, waitErr
	}

	var successCount int
	if err = db.QueryRowContext(context.Background(), "SELECT COUNT(1) FROM seckill_success WHERE activity_id = ?", activityID).Scan(&successCount); err != nil {
		return metricSummary{}, err
	}

	oversold := 0
	if successCount > stock {
		oversold = successCount - stock
	}

	summary := buildSummary("seckill_submit", cfg.requests, cfg.concurrency, success, failure, durations, elapsed, "")
	summary.ExpectedStock = stock
	summary.FinalSuccessCount = successCount
	summary.OversoldCount = oversold
	return summary, nil
}

func runSeckillDuplicate(cfg benchConfig) (metricSummary, error) {
	seckillClient, err := seckillservice.NewClient("seckill.service", client.WithHostPorts(cfg.seckillAddr))
	if err != nil {
		return metricSummary{}, err
	}
	db, err := sql.Open("mysql", cfg.mysqlDSN)
	if err != nil {
		return metricSummary{}, err
	}
	defer db.Close()

	userCount := cfg.requests
	activityID, err := createSeckillActivity(seckillClient, userCount, 109)
	if err != nil {
		return metricSummary{}, err
	}

	totalRequests := userCount * cfg.duplicateFan
	durations, success, failure, elapsed := runWorkers(totalRequests, cfg.concurrency, func(i int) error {
		userID := int64(4_000_000 + (i % userCount))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := seckillClient.SubmitSeckill(ctx, &seckillv1.SubmitSeckillReq{
			ActivityId: activityID,
			UserId:     userID,
		})
		return err
	})

	if err = waitForSeckillSettlement(db, activityID, time.Now().Add(60*time.Second)); err != nil {
		return metricSummary{}, err
	}

	var duplicateUsers int
	if err = db.QueryRowContext(context.Background(), `
SELECT COUNT(1) FROM (
  SELECT user_id
  FROM seckill_request
  WHERE activity_id = ? AND status IN ('PROCESSING','QUALIFIED','SUCCESS')
  GROUP BY user_id
  HAVING COUNT(1) > 1
) t`, activityID).Scan(&duplicateUsers); err != nil {
		return metricSummary{}, err
	}

	summary := buildSummary("seckill_duplicate_guard", totalRequests, cfg.concurrency, success, failure, durations, elapsed, "")
	summary.DuplicateUsers = duplicateUsers
	summary.DuplicateRate = float64(duplicateUsers) / float64(userCount)
	summary.FinalSuccessCount = userCount - duplicateUsers
	return summary, nil
}

func seedProductAndStock(productClient productservice.Client, inventoryClient inventoryservice.Client, stock int, price int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runSuffix := time.Now().UnixNano()
	createResp, err := productClient.CreateProduct(ctx, &productv1.CreateProductReq{
		Product: &productv1.Product{
			Name:         fmt.Sprintf("bench-product-%d", runSuffix),
			Description:  "benchmark product",
			Picture:      "https://example.com/bench.png",
			Price:        price,
			Currency:     "CNY",
			Categories:   []string{"benchmark"},
			InStock:      true,
			MerchantID:   10001,
			MerchantName: "benchmark",
		},
	})
	if err != nil {
		return 0, err
	}

	adjustResp, err := inventoryClient.AdjustStock(ctx, &inventoryv1.AdjustStockReq{
		Reason: fmt.Sprintf("bench_seed_%d", runSuffix),
		Items: []*inventoryv1.StockItem{{
			ProductId: createResp.GetId(),
			Quantity:  int32(stock),
		}},
	})
	if err != nil {
		return 0, err
	}
	if adjustResp.GetStatusCode() != 0 {
		return 0, fmt.Errorf("adjust stock failed: code=%d msg=%s", adjustResp.GetStatusCode(), adjustResp.GetStatusMsg())
	}
	return createResp.GetId(), nil
}

func createSeckillActivity(client seckillservice.Client, stock int, price int64) (int64, error) {
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.CreateActivity(ctx, &seckillv1.CreateActivityReq{
		ActivityName: fmt.Sprintf("bench-seckill-%d", now.UnixNano()),
		ProductId:    now.UnixNano()%1_000_000 + 10_000,
		SkuId:        now.UnixNano()%1_000_000 + 20_000,
		SeckillPrice: price,
		TotalStock:   int32(stock),
		StartTime:    now.Add(-time.Minute).Unix(),
		EndTime:      now.Add(10 * time.Minute).Unix(),
		Status:       "ONLINE",
		LimitPerUser: 1,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetActivityId(), nil
}

func waitForSeckillSettlement(db *sql.DB, activityID int64, deadline time.Time) error {
	for time.Now().Before(deadline) {
		var total int
		err := db.QueryRowContext(context.Background(), "SELECT COUNT(1) FROM seckill_request WHERE activity_id = ?", activityID).Scan(&total)
		if err != nil {
			return err
		}
		if total == 0 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var pending int
		err = db.QueryRowContext(context.Background(), "SELECT COUNT(1) FROM seckill_request WHERE activity_id = ? AND status = 'PROCESSING'", activityID).Scan(&pending)
		if err != nil {
			return err
		}
		if pending == 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting seckill activity %d settle", activityID)
}

func runWorkers(total, concurrency int, fn func(i int) error) ([]float64, int64, int64, time.Duration) {
	var success int64
	var failure int64
	durations := make([]float64, 0, total)
	var mu sync.Mutex
	jobs := make(chan int)
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				reqStart := time.Now()
				err := fn(job)
				elapsed := time.Since(reqStart).Seconds() * 1000
				mu.Lock()
				durations = append(durations, elapsed)
				mu.Unlock()
				if err != nil {
					atomic.AddInt64(&failure, 1)
					continue
				}
				atomic.AddInt64(&success, 1)
			}
		}()
	}

	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return durations, success, failure, time.Since(start)
}

func buildSummary(name string, requests, concurrency int, success, failure int64, durations []float64, elapsed time.Duration, extra string) metricSummary {
	sort.Float64s(durations)
	total := success + failure
	successRate := 0.0
	if total > 0 {
		successRate = float64(success) / float64(total)
	}
	rps := 0.0
	if elapsed > 0 {
		rps = float64(total) / elapsed.Seconds()
	}
	return metricSummary{
		Scenario:         name,
		Requests:         requests,
		Concurrency:      concurrency,
		Success:          success,
		Failure:          failure,
		SuccessRate:      successRate,
		RPS:              rps,
		P50Millis:        percentile(durations, 0.50),
		P90Millis:        percentile(durations, 0.90),
		P99Millis:        percentile(durations, 0.99),
		TotalDurationMS:  elapsed.Seconds() * 1000,
		ExtraDescription: extra,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := int(math.Ceil(float64(len(sorted))*p)) - 1
	if pos < 0 {
		pos = 0
	}
	if pos >= len(sorted) {
		pos = len(sorted) - 1
	}
	return sorted[pos]
}
