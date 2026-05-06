//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/apache/rocketmq-clients/golang"
	"github.com/cloudwego/kitex/client"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const (
	seckillHostPort  = "127.0.0.1:8098"
	orderHostPort    = "127.0.0.1:8095"
	paymentHostPort  = "127.0.0.1:8092"
	mockWechatURL    = "http://127.0.0.1:8888"
	mysqlDSN         = "root:root@tcp(127.0.0.1:13306)/douyinmall?charset=utf8mb4&parseTime=True&loc=Local"
	redisAddr        = "127.0.0.1:16379"
	rocketMQEndpoint = "127.0.0.1:8081"
)

func TestSeckillComposeEndToEnd(t *testing.T) {
	ensureServiceReady(t, seckillHostPort)
	ensureServiceReady(t, orderHostPort)
	ensureServiceReady(t, paymentHostPort)
	ensureHTTPReady(t, mockWechatURL+"/mock/orders")

	seckillClient, err := seckillservice.NewClient("seckill.service", client.WithHostPorts(seckillHostPort))
	require.NoError(t, err)

	orderClient, err := orderservice.NewClient("order.service", client.WithHostPorts(orderHostPort))
	require.NoError(t, err)

	payClient, err := paymentservice.NewClient("payment.service", client.WithHostPorts(paymentHostPort))
	require.NoError(t, err)

	now := time.Now()
	userID := now.UnixNano()%1_000_000_000 + 10_000
	productID := now.UnixNano()%1_000_000 + 20_000
	skuID := productID + 1
	price := int64(199)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	createResp, err := seckillClient.CreateActivity(ctx, &seckillv1.CreateActivityReq{
		ActivityName: "compose-e2e-seckill",
		ProductId:    productID,
		SkuId:        skuID,
		SeckillPrice: price,
		TotalStock:   1,
		StartTime:    now.Add(-time.Minute).Unix(),
		EndTime:      now.Add(10 * time.Minute).Unix(),
		Status:       "ONLINE",
		LimitPerUser: 1,
	})
	require.NoError(t, err)
	require.NotZero(t, createResp.GetActivityId())

	submitResp, err := seckillClient.SubmitSeckill(ctx, &seckillv1.SubmitSeckillReq{
		ActivityId: createResp.GetActivityId(),
		UserId:     userID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, submitResp.GetRequestNo())
	require.Equal(t, "PROCESSING", submitResp.GetStatus())

	var resultResp *seckillv1.GetSeckillResultResp
	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		resultResp, err = seckillClient.GetSeckillResult(pollCtx, &seckillv1.GetSeckillResultReq{
			RequestNo: submitResp.GetRequestNo(),
		})
		return err == nil && resultResp.GetStatus() == "SUCCESS" && resultResp.GetOrderId() != 0
	}, 20*time.Second, 500*time.Millisecond, "seckill should succeed with order id for payment")

	orderResp, err := orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: resultResp.GetOrderId()})
	require.NoError(t, err)
	require.NotNil(t, orderResp.GetOrder())
	require.Equal(t, orderv1.OrderStatus_ORDER_STATUS_CREATED, orderResp.GetOrder().GetOrderStatus())
	require.Equal(t, "SECKILL", orderResp.GetOrder().GetOrderKind())
	require.Equal(t, createResp.GetActivityId(), orderResp.GetOrder().GetActivityId())
	require.Len(t, orderResp.GetOrder().GetItems(), 1)
	require.Equal(t, price, orderResp.GetOrder().GetTotalAmount())

	_, err = payClient.NativePrepay(ctx, &paymentv1.NativePrePayRequest{
		Amt: &paymentv1.Amount{
			Total:    price,
			Currency: "CNY",
		},
		BizTradeNo:  fmt.Sprintf("%d", resultResp.GetOrderId()),
		Description: "compose e2e seckill payment",
	})
	require.NoError(t, err)

	resp, err := http.Post(fmt.Sprintf("%s/mock/pay/%d", mockWechatURL, resultResp.GetOrderId()), "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		paymentResp, payErr := payClient.GetPayment(pollCtx, &paymentv1.GetPaymentRequest{
			BizTradeNo: fmt.Sprintf("%d", resultResp.GetOrderId()),
		})
		return payErr == nil && paymentResp.GetStatus() == paymentv1.PaymentStatus_PaymentStatusSuccess
	}, 20*time.Second, 500*time.Millisecond, "payment status should become success")

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		orderResp, err = orderClient.GetOrder(pollCtx, &orderv1.GetOrderReq{OrderId: resultResp.GetOrderId()})
		return err == nil && orderResp.GetOrder().GetOrderStatus() == orderv1.OrderStatus_ORDER_STATUS_PAID
	}, 20*time.Second, 500*time.Millisecond, "seckill order should become paid after mock payment callback")
}

func TestSeckillCreateOrderFailureCompensates(t *testing.T) {
	ensureServiceReady(t, seckillHostPort)
	ensureServiceReady(t, orderHostPort)

	seckillClient, err := seckillservice.NewClient("seckill.service", client.WithHostPorts(seckillHostPort))
	require.NoError(t, err)

	db := openMySQL(t)
	defer db.Close()

	rdb := openRedis(t)
	defer func() { _ = rdb.Close() }()

	now := time.Now()
	userID := now.UnixNano()%1_000_000_000 + 90_000
	productID := now.UnixNano()%1_000_000 + 30_000
	skuID := productID + 1
	requestNo := fmt.Sprintf("bad-order-%d", now.UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	createResp, err := seckillClient.CreateActivity(ctx, &seckillv1.CreateActivityReq{
		ActivityName: "compose-e2e-seckill-fail",
		ProductId:    productID,
		SkuId:        skuID,
		SeckillPrice: 259,
		TotalStock:   1,
		StartTime:    now.Add(-time.Minute).Unix(),
		EndTime:      now.Add(10 * time.Minute).Unix(),
		Status:       "ONLINE",
		LimitPerUser: 1,
	})
	require.NoError(t, err)
	require.NotZero(t, createResp.GetActivityId())

	stockRedisKey := fmt.Sprintf("seckill:stock:%d", createResp.GetActivityId())
	userRedisKey := fmt.Sprintf("seckill:user:%d:%d", createResp.GetActivityId(), userID)
	statusRedisKey := fmt.Sprintf("seckill:req:status:%s", requestNo)
	dataRedisKey := fmt.Sprintf("seckill:req:data:%s", requestNo)

	require.NoError(t, rdb.DecrBy(ctx, stockRedisKey, 1).Err())
	require.NoError(t, rdb.Set(ctx, userRedisKey, requestNo, time.Hour).Err())
	require.NoError(t, rdb.Set(ctx, statusRedisKey, "PROCESSING", time.Hour).Err())

	resultPayload, err := json.Marshal(map[string]any{
		"requestNo": requestNo,
		"status":    "PROCESSING",
	})
	require.NoError(t, err)
	require.NoError(t, rdb.Set(ctx, dataRedisKey, resultPayload, time.Hour).Err())

	_, err = db.ExecContext(ctx,
		"INSERT INTO seckill_request (request_no, activity_id, user_id, status, fail_reason, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NOW(), NOW())",
		requestNo, createResp.GetActivityId(), userID, "PROCESSING", "",
	)
	require.NoError(t, err)

	payload, err := json.Marshal(map[string]any{
		"request_no":    requestNo,
		"activity_id":   createResp.GetActivityId(),
		"user_id":       userID,
		"product_id":    productID,
		"sku_id":        skuID,
		"seckill_price": 259,
	})
	require.NoError(t, err)

	producer := openRocketMQProducer(t)
	defer func() { _ = producer.GracefulStop() }()

	msg := &golang.Message{
				Topic: "seckill_request",
		Body:  payload,
	}
	msg.SetKeys(requestNo)
	_, err = producer.Send(ctx, msg)
	require.NoError(t, err)

	var resultResp *seckillv1.GetSeckillResultResp
	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		resultResp, err = seckillClient.GetSeckillResult(pollCtx, &seckillv1.GetSeckillResultReq{
			RequestNo: requestNo,
		})
		return err == nil && resultResp.GetStatus() == "FAILED" && resultResp.GetFailReason() == "CREATE_ORDER_FAIL"
	}, 20*time.Second, 500*time.Millisecond, "seckill request should fail and be compensated")

	require.Eventually(t, func() bool {
		val, redisErr := rdb.Get(context.Background(), stockRedisKey).Int64()
		return redisErr == nil && val == 1
	}, 10*time.Second, 500*time.Millisecond, "redis stock should be restored after compensation")

	exists, err := rdb.Exists(ctx, userRedisKey).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists)

	var requestStatus, failReason string
	err = db.QueryRowContext(ctx,
		"SELECT status, fail_reason FROM seckill_request WHERE request_no = ?",
		requestNo,
	).Scan(&requestStatus, &failReason)
	require.NoError(t, err)
	require.Equal(t, "FAILED", requestStatus)
	require.Equal(t, "CREATE_ORDER_FAIL", failReason)

	var availableStock int32
	err = db.QueryRowContext(ctx,
		"SELECT available_stock FROM seckill_activity WHERE id = ?",
		createResp.GetActivityId(),
	).Scan(&availableStock)
	require.NoError(t, err)
	require.EqualValues(t, 1, availableStock)

	var qualificationCount int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(1) FROM seckill_qualification WHERE activity_id = ? AND user_id = ?",
		createResp.GetActivityId(), userID,
	).Scan(&qualificationCount)
	require.NoError(t, err)
	require.Zero(t, qualificationCount)
}

func ensureServiceReady(t *testing.T, addr string) {
	t.Helper()
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 15*time.Second, 500*time.Millisecond, "service %s is not reachable", addr)
}

func ensureHTTPReady(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	require.Eventually(t, func() bool {
		resp, err := client.Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 15*time.Second, 500*time.Millisecond, "http endpoint %s is not reachable", url)
}

func openMySQL(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", mysqlDSN)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.PingContext(checkCtx) == nil
	}, 15*time.Second, 500*time.Millisecond, "mysql is not reachable")

	return db
}

func openRedis(t *testing.T) *redis.Client {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	require.Eventually(t, func() bool {
		checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return rdb.Ping(checkCtx).Err() == nil
	}, 15*time.Second, 500*time.Millisecond, "redis is not reachable")

	return rdb
}

func openRocketMQProducer(t *testing.T) golang.Producer {
	t.Helper()

	var producer golang.Producer
	require.Eventually(t, func() bool {
		var err error
		producer, err = golang.NewProducer(
			&golang.Config{Endpoint: rocketMQEndpoint},
		golang.WithTopics("seckill_request"),
		)
		if err != nil {
			return false
		}
		if err = producer.Start(); err != nil {
			return false
		}
		return true
	}, 15*time.Second, 500*time.Millisecond, "rocketmq producer is not reachable")

	return producer
}
