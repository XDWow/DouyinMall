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
	stock         int
	users         int
	targetQPS     int
	duplicatePct  int
	hotPct        int
	duplicateFan  int
	settleTimeout time.Duration
}

type metricSummary struct {
	Scenario            string         `json:"scenario"`
	Requests            int            `json:"requests"`
	Concurrency         int            `json:"concurrency"`
	Success             int64          `json:"success"`
	Failure             int64          `json:"failure"`
	SuccessRate         float64        `json:"success_rate"`
	RPS                 float64        `json:"rps"`
	P50Millis           float64        `json:"p50_ms"`
	P90Millis           float64        `json:"p90_ms"`
	P99Millis           float64        `json:"p99_ms"`
	TotalDurationMS     float64        `json:"total_duration_ms"`
	SubmitDurationMS    float64        `json:"submit_duration_ms,omitempty"`
	SettleDurationMS    float64        `json:"settle_duration_ms,omitempty"`
	EndToEndDurationMS  float64        `json:"end_to_end_duration_ms,omitempty"`
	ExtraDescription    string         `json:"extra_description,omitempty"`
	CorrectedRate       float64        `json:"corrected_rate"`
	MisclosedOrders     int            `json:"misclosed_orders"`
	OversoldCount       int            `json:"oversold_count"`
	DuplicateUsers      int            `json:"duplicate_users"`
	DuplicateRate       float64        `json:"duplicate_rate"`
	ExpectedStock       int            `json:"expected_stock"`
	FinalSuccessCount   int            `json:"final_success_count"`
	FinalAvailableStock int            `json:"final_available_stock"`
	ActivityID          int64          `json:"activity_id,omitempty"`
	UserPool            int            `json:"user_pool,omitempty"`
	TargetQPS           int            `json:"target_qps,omitempty"`
	RequestedDuplicate  int            `json:"requested_duplicate_percent,omitempty"`
	EffectiveDuplicate  float64        `json:"effective_duplicate_percent,omitempty"`
	DuplicateRequests   int            `json:"duplicate_requests,omitempty"`
	RequestedHot        int            `json:"requested_hot_percent,omitempty"`
	HotRequests         int            `json:"hot_requests,omitempty"`
	ColdRequests        int            `json:"cold_requests,omitempty"`
	HotActivityID       int64          `json:"hot_activity_id,omitempty"`
	ColdActivityID      int64          `json:"cold_activity_id,omitempty"`
	HotExpectedStock    int            `json:"hot_expected_stock,omitempty"`
	ColdExpectedStock   int            `json:"cold_expected_stock,omitempty"`
	HotSuccessCount     int            `json:"hot_final_success_count,omitempty"`
	ColdSuccessCount    int            `json:"cold_final_success_count,omitempty"`
	StatusCounts        map[string]int `json:"status_counts,omitempty"`
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
	flag.IntVar(&cfg.stock, "stock", 0, "seckill stock; defaults to requests/4 with a minimum of 50")
	flag.IntVar(&cfg.users, "users", 0, "available user pool for seckill traffic; defaults to requests")
	flag.IntVar(&cfg.targetQPS, "target_qps", 0, "target request rate for paced submission; 0 means no rate limit")
	flag.IntVar(&cfg.duplicatePct, "duplicate_percent", 0, "duplicate request percent for seckill traffic")
	flag.IntVar(&cfg.hotPct, "hot_percent", 100, "hot activity traffic percent for seckill traffic")
	flag.IntVar(&cfg.duplicateFan, "duplicate_fan", 5, "duplicate requests per user")
	settleTimeoutSec := flag.Int("settle_timeout_sec", 300, "max seconds to wait for async seckill settlement")
	flag.Parse()
	cfg.settleTimeout = time.Duration(*settleTimeoutSec) * time.Second
	if cfg.users <= 0 {
		cfg.users = cfg.requests
	}
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

	productID, skuID, err := seedProductAndStock(productClient, inventoryClient, cfg.requests+20, 299)
	if err != nil {
		return metricSummary{}, err
	}

	durations, success, failure, elapsed := runWorkers(cfg.requests, cfg.concurrency, 0, func(i int) error {
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

	productID, skuID, err := seedProductAndStock(productClient, inventoryClient, cfg.requests+20, 399)
	if err != nil {
		return metricSummary{}, err
	}

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

	stock := seckillStock(cfg)
	workload, err := buildSeckillWorkload(cfg.requests, cfg.users, cfg.duplicatePct, cfg.hotPct)
	if err != nil {
		return metricSummary{}, err
	}

	hotStock, coldStock := splitStockByTraffic(stock, cfg.hotPct)
	if hotStock <= 0 {
		hotStock = stock
	}

	hotActivityID, err := createSeckillActivity(seckillClient, hotStock, 99)
	if err != nil {
		return metricSummary{}, err
	}
	activityIDs := []int64{hotActivityID}
	coldActivityID := int64(0)
	if coldStock > 0 {
		coldActivityID, err = createSeckillActivity(seckillClient, coldStock, 109)
		if err != nil {
			return metricSummary{}, err
		}
		activityIDs = append(activityIDs, coldActivityID)
	}

	workStart := time.Now()
	durations, success, failure, elapsed := runWorkers(len(workload.Requests), cfg.concurrency, cfg.targetQPS, func(i int) error {
		spec := workload.Requests[i]
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		activityID := hotActivityID
		if spec.ActivityIndex == 1 && coldActivityID != 0 {
			activityID = coldActivityID
		}
		resp, err := seckillClient.SubmitSeckill(ctx, &seckillv1.SubmitSeckillReq{
			ActivityId: activityID,
			UserId:     spec.UserID,
		})
		if err != nil {
			return err
		}
		if resp.GetStatus() == "FAILED" {
			return fmt.Errorf("submit rejected: %s", resp.GetMessage())
		}
		return nil
	})

	settleStart := time.Now()
	waitErr := waitForSeckillSettlement(db, activityIDs, int(success), time.Now().Add(cfg.settleTimeout))
	settleElapsed := time.Since(settleStart)
	if waitErr != nil {
		return metricSummary{}, waitErr
	}

	hotSuccessCount, hotAvailableStock, err := querySeckillActivityOutcome(db, hotActivityID)
	if err != nil {
		return metricSummary{}, err
	}
	coldSuccessCount := 0
	coldAvailableStock := 0
	if coldActivityID != 0 {
		coldSuccessCount, coldAvailableStock, err = querySeckillActivityOutcome(db, coldActivityID)
		if err != nil {
			return metricSummary{}, err
		}
	}
	statusCounts, err := querySeckillStatusCounts(db, activityIDs)
	if err != nil {
		return metricSummary{}, err
	}

	successCount := hotSuccessCount + coldSuccessCount
	finalAvailableStock := hotAvailableStock + coldAvailableStock
	oversold := 0
	if successCount > stock {
		oversold = successCount - stock
	}

	extra := ""
	if workload.EffectiveDuplicatePct != float64(workload.RequestedDuplicatePct) {
		extra = fmt.Sprintf("effective duplicate percent adjusted to %.2f%% because user pool=%d is smaller than requested unique demand", workload.EffectiveDuplicatePct, cfg.users)
	}

	summary := buildSummary("seckill_submit", cfg.requests, cfg.concurrency, success, failure, durations, elapsed, extra)
	summary.ActivityID = hotActivityID
	summary.SubmitDurationMS = elapsed.Seconds() * 1000
	summary.SettleDurationMS = settleElapsed.Seconds() * 1000
	summary.EndToEndDurationMS = time.Since(workStart).Seconds() * 1000
	summary.ExpectedStock = stock
	summary.FinalSuccessCount = successCount
	summary.FinalAvailableStock = finalAvailableStock
	summary.OversoldCount = oversold
	summary.StatusCounts = statusCounts
	summary.UserPool = cfg.users
	summary.TargetQPS = cfg.targetQPS
	summary.RequestedDuplicate = workload.RequestedDuplicatePct
	summary.EffectiveDuplicate = workload.EffectiveDuplicatePct
	summary.DuplicateRequests = workload.DuplicateRequests
	summary.RequestedHot = workload.RequestedHotTrafficPct
	summary.HotRequests = workload.HotRequests
	summary.ColdRequests = workload.ColdRequests
	summary.HotActivityID = hotActivityID
	summary.ColdActivityID = coldActivityID
	summary.HotExpectedStock = hotStock
	summary.ColdExpectedStock = coldStock
	summary.HotSuccessCount = hotSuccessCount
	summary.ColdSuccessCount = coldSuccessCount
	return summary, nil
}

func seckillStock(cfg benchConfig) int {
	if cfg.stock > 0 {
		return cfg.stock
	}
	stock := cfg.requests / 4
	if stock < 50 {
		stock = 50
	}
	return stock
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
	workStart := time.Now()
	durations, success, failure, elapsed := runWorkers(totalRequests, cfg.concurrency, 0, func(i int) error {
		userID := int64(4_000_000 + (i % userCount))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := seckillClient.SubmitSeckill(ctx, &seckillv1.SubmitSeckillReq{
			ActivityId: activityID,
			UserId:     userID,
		})
		if err != nil {
			return err
		}
		if resp.GetStatus() == "FAILED" {
			return fmt.Errorf("submit rejected: %s", resp.GetMessage())
		}
		return nil
	})

	settleStart := time.Now()
	if err = waitForSeckillSettlement(db, []int64{activityID}, int(success), time.Now().Add(cfg.settleTimeout)); err != nil {
		return metricSummary{}, err
	}
	settleElapsed := time.Since(settleStart)

	var duplicateUsers int
	if err = db.QueryRowContext(context.Background(), `
SELECT COUNT(1) FROM (
  SELECT user_id
  FROM seckill_qualification
  WHERE activity_id = ?
  GROUP BY user_id
  HAVING COUNT(1) > 1
) t`, activityID).Scan(&duplicateUsers); err != nil {
		return metricSummary{}, err
	}
	var finalAvailableStock int
	if err = db.QueryRowContext(context.Background(), "SELECT available_stock FROM seckill_activity WHERE id = ?", activityID).Scan(&finalAvailableStock); err != nil {
		return metricSummary{}, err
	}
	statusCounts, err := querySeckillStatusCounts(db, []int64{activityID})
	if err != nil {
		return metricSummary{}, err
	}

	summary := buildSummary("seckill_duplicate_guard", totalRequests, cfg.concurrency, success, failure, durations, elapsed, "")
	summary.ActivityID = activityID
	summary.SubmitDurationMS = elapsed.Seconds() * 1000
	summary.SettleDurationMS = settleElapsed.Seconds() * 1000
	summary.EndToEndDurationMS = time.Since(workStart).Seconds() * 1000
	summary.ExpectedStock = userCount
	summary.DuplicateUsers = duplicateUsers
	summary.DuplicateRate = float64(duplicateUsers) / float64(userCount)
	summary.FinalSuccessCount = userCount - duplicateUsers
	summary.FinalAvailableStock = finalAvailableStock
	summary.StatusCounts = statusCounts
	return summary, nil
}

func seedProductAndStock(productClient productservice.Client, inventoryClient inventoryservice.Client, stock int, price int64) (int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runSuffix := time.Now().UnixNano()
	skuID := runSuffix%1_000_000_000 + 1_000_000_000
	createResp, err := productClient.CreateProduct(ctx, &productv1.CreateProductReq{
		Product: &productv1.Product{
			Name:         fmt.Sprintf("bench-product-%d", runSuffix),
			Description:  "benchmark product",
			Picture:      "https://example.com/bench.png",
			SkuId:        skuID,
			Price:        price,
			Currency:     "CNY",
			Categories:   []string{"benchmark"},
			InStock:      true,
			MerchantID:   10001,
			MerchantName: "benchmark",
		},
	})
	if err != nil {
		return 0, 0, err
	}

	adjustResp, err := inventoryClient.AdjustStock(ctx, &inventoryv1.AdjustStockReq{
		Reason: fmt.Sprintf("bench_seed_%d", runSuffix),
		Items: []*inventoryv1.StockItem{{
			ProductId: createResp.GetId(),
			Quantity:  int32(stock),
		}},
	})
	if err != nil {
		return 0, 0, err
	}
	if adjustResp.GetStatusCode() != 0 {
		return 0, 0, fmt.Errorf("adjust stock failed: code=%d msg=%s", adjustResp.GetStatusCode(), adjustResp.GetStatusMsg())
	}
	return createResp.GetId(), skuID, nil
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

func waitForSeckillSettlement(db *sql.DB, activityIDs []int64, expectedRequests int, deadline time.Time) error {
	for time.Now().Before(deadline) {
		total, err := querySeckillRequestCount(db, activityIDs, "")
		if err != nil {
			return err
		}
		if total < expectedRequests {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		pending, err := querySeckillRequestCount(db, activityIDs, "AND status IN ('PROCESSING','ORDER_CREATING')")
		if err != nil {
			return err
		}
		if pending == 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	statusCounts, err := querySeckillStatusCounts(db, activityIDs)
	if err != nil {
		return fmt.Errorf("timeout waiting seckill activities %v settle and query status summary failed: %w", activityIDs, err)
	}
	return fmt.Errorf("timeout waiting seckill activities %v settle, expectedRequests=%d statusCounts=%v", activityIDs, expectedRequests, statusCounts)
}

func querySeckillStatusCounts(db *sql.DB, activityIDs []int64) (map[string]int, error) {
	statusCounts := map[string]int{}
	for _, activityID := range activityIDs {
		rows, err := db.QueryContext(context.Background(), "SELECT status, COUNT(1) FROM seckill_request WHERE activity_id = ? GROUP BY status", activityID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var status string
			var count int
			if scanErr := rows.Scan(&status, &count); scanErr != nil {
				_ = rows.Close()
				return nil, scanErr
			}
			statusCounts[status] += count
		}
		if err = rows.Close(); err != nil {
			return nil, err
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
	}
	return statusCounts, nil
}

func querySeckillActivityOutcome(db *sql.DB, activityID int64) (successCount int, availableStock int, err error) {
	if err = db.QueryRowContext(context.Background(), "SELECT COUNT(1) FROM seckill_qualification WHERE activity_id = ?", activityID).Scan(&successCount); err != nil {
		return 0, 0, err
	}
	if err = db.QueryRowContext(context.Background(), "SELECT available_stock FROM seckill_activity WHERE id = ?", activityID).Scan(&availableStock); err != nil {
		return 0, 0, err
	}
	return successCount, availableStock, nil
}

func querySeckillRequestCount(db *sql.DB, activityIDs []int64, extraCondition string) (int, error) {
	total := 0
	for _, activityID := range activityIDs {
		var count int
		query := "SELECT COUNT(1) FROM seckill_request WHERE activity_id = ? " + extraCondition
		if err := db.QueryRowContext(context.Background(), query, activityID).Scan(&count); err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func runWorkers(total, concurrency, targetQPS int, fn func(i int) error) ([]float64, int64, int64, time.Duration) {
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

	dispatchStart := time.Now()
	for i := 0; i < total; i++ {
		if targetQPS > 0 {
			scheduledAt := dispatchStart.Add(time.Duration(i) * time.Second / time.Duration(targetQPS))
			if sleep := time.Until(scheduledAt); sleep > 0 {
				time.Sleep(sleep)
			}
		}
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
