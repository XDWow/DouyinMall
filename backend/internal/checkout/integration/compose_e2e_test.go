//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	checkoutv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/cloudwego/kitex/client"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const (
	checkoutHostPort  = "127.0.0.1:8086"
	productHostPort   = "127.0.0.1:8096"
	inventoryHostPort = "127.0.0.1:8094"
	orderHostPort     = "127.0.0.1:8095"
	paymentHostPort   = "127.0.0.1:8092"
	mockWechatURL     = "http://127.0.0.1:8888"
	mysqlDSN          = "root:root@tcp(127.0.0.1:13306)/douyinmall?charset=utf8mb4&parseTime=True&loc=Local"
	redisAddr         = "127.0.0.1:16379"
)

func TestCheckoutComposeEndToEnd(t *testing.T) {
	ensureServiceReady(t, checkoutHostPort)
	ensureServiceReady(t, productHostPort)
	ensureServiceReady(t, inventoryHostPort)
	ensureServiceReady(t, orderHostPort)
	ensureServiceReady(t, paymentHostPort)
	ensureHTTPReady(t, mockWechatURL+"/mock/orders")

	checkoutClient, err := checkoutservice.NewClient("checkout.service", client.WithHostPorts(checkoutHostPort))
	require.NoError(t, err)

	productClient, err := productservice.NewClient("product.service", client.WithHostPorts(productHostPort))
	require.NoError(t, err)

	inventoryClient, err := inventoryservice.NewClient("inventory.service", client.WithHostPorts(inventoryHostPort))
	require.NoError(t, err)

	orderClient, err := orderservice.NewClient("order.service", client.WithHostPorts(orderHostPort))
	require.NoError(t, err)

	payClient, err := paymentservice.NewClient("payment.service", client.WithHostPorts(paymentHostPort))
	require.NoError(t, err)

	now := time.Now()
	userID := now.UnixNano()%1_000_000_000 + 50_000

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createProductResp, err := productClient.CreateProduct(ctx, &productv1.CreateProductReq{
		Product: &productv1.Product{
			Name:         fmt.Sprintf("compose-e2e-product-%d", userID),
			Description:  "compose checkout e2e product",
			Picture:      "https://example.com/product.png",
			Price:        299,
			Currency:     "CNY",
			Categories:   []string{"integration"},
			InStock:      true,
			MerchantID:   10001,
			MerchantName: "integration-test",
		},
	})
	require.NoError(t, err)
	require.NotZero(t, createProductResp.GetId())

	adjustResp, err := inventoryClient.AdjustStock(ctx, &inventoryv1.AdjustStockReq{
		Reason: "compose_e2e_seed",
		Items: []*inventoryv1.StockItem{
			{
				ProductId: createProductResp.GetId(),
				Quantity:  2,
			},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 0, adjustResp.GetStatusCode())

	placeResp, err := checkoutClient.PlaceOrder(ctx, &checkoutv1.PlaceOrderReq{
		UserId: userID,
		Items: []*checkoutv1.CheckoutItem{
			{
				ProductId: createProductResp.GetId(),
				Quantity:  1,
			},
		},
		Address: &checkoutv1.Address{
			ReceiverName: "compose buyer",
			Phone:        "13800138000",
			Province:     "Shanghai",
			City:         "Shanghai",
			District:     "Pudong",
			Street:       "No.1 Integration Road",
			ZipCode:      "200120",
		},
		PaymentMethod:  "WECHAT_NATIVE",
		Currency:       "CNY",
		Remark:         "please call before delivery",
		ExpectedAmount: 299,
	})
	require.NoError(t, err)
	require.NotZero(t, placeResp.GetOrderId())
	require.NotEmpty(t, placeResp.GetPaymentUrl())
	require.EqualValues(t, 299, placeResp.GetTotalAmount())

	orderResp, err := orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: placeResp.GetOrderId()})
	require.NoError(t, err)
	require.NotNil(t, orderResp.GetOrder())
	require.Equal(t, userID, orderResp.GetOrder().GetUserId())
	require.Equal(t, orderv1.OrderStatus_ORDER_STATUS_CREATED, orderResp.GetOrder().GetOrderStatus())
	require.Equal(t, "DIRECT_BUY", orderResp.GetOrder().GetOrderKind())
	require.Equal(t, "please call before delivery", orderResp.GetOrder().GetRemark())
	require.Len(t, orderResp.GetOrder().GetItems(), 1)
	require.EqualValues(t, 299, orderResp.GetOrder().GetTotalAmount())

	batchResp, err := inventoryClient.BatchGetInventory(ctx, &inventoryv1.BatchGetInventoryReq{
		ProductIds: []int64{createProductResp.GetId()},
	})
	require.NoError(t, err)
	require.Len(t, batchResp.GetInventories(), 1)
	require.EqualValues(t, 1, batchResp.GetInventories()[0].GetAvailableStock())

	retryPayResp, err := checkoutClient.PayOrder(ctx, &checkoutv1.PayOrderReq{
		UserId:  userID,
		OrderId: placeResp.GetOrderId(),
	})
	require.NoError(t, err)
	require.Equal(t, placeResp.GetOrderId(), retryPayResp.GetOrderId())
	require.EqualValues(t, 299, retryPayResp.GetTotalAmount())
	require.NotEmpty(t, retryPayResp.GetPaymentUrl())

	resp, err := http.Post(fmt.Sprintf("%s/mock/pay/%d", mockWechatURL, placeResp.GetOrderId()), "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		paymentResp, payErr := payClient.GetPayment(pollCtx, &paymentv1.GetPaymentRequest{
			BizTradeNo: fmt.Sprintf("%d", placeResp.GetOrderId()),
		})
		return payErr == nil && paymentResp.GetStatus() == paymentv1.PaymentStatus_PaymentStatusSuccess
	}, 20*time.Second, 500*time.Millisecond, "payment status should become success")

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		orderResp, err = orderClient.GetOrder(pollCtx, &orderv1.GetOrderReq{OrderId: placeResp.GetOrderId()})
		return err == nil && orderResp.GetOrder().GetOrderStatus() == orderv1.OrderStatus_ORDER_STATUS_PAID
	}, 20*time.Second, 500*time.Millisecond, "checkout order should become paid after mock payment")
}

func TestCheckoutTimeoutCancelRestoresStock(t *testing.T) {
	ensureServiceReady(t, checkoutHostPort)
	ensureServiceReady(t, productHostPort)
	ensureServiceReady(t, inventoryHostPort)
	ensureServiceReady(t, orderHostPort)
	ensureServiceReady(t, paymentHostPort)

	checkoutClient, err := checkoutservice.NewClient("checkout.service", client.WithHostPorts(checkoutHostPort))
	require.NoError(t, err)

	productClient, err := productservice.NewClient("product.service", client.WithHostPorts(productHostPort))
	require.NoError(t, err)

	inventoryClient, err := inventoryservice.NewClient("inventory.service", client.WithHostPorts(inventoryHostPort))
	require.NoError(t, err)

	orderClient, err := orderservice.NewClient("order.service", client.WithHostPorts(orderHostPort))
	require.NoError(t, err)

	now := time.Now()
	userID := now.UnixNano()%1_000_000_000 + 80_000

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createProductResp, err := productClient.CreateProduct(ctx, &productv1.CreateProductReq{
		Product: &productv1.Product{
			Name:         fmt.Sprintf("compose-timeout-product-%d", userID),
			Description:  "compose checkout timeout product",
			Picture:      "https://example.com/product-timeout.png",
			Price:        399,
			Currency:     "CNY",
			Categories:   []string{"integration"},
			InStock:      true,
			MerchantID:   10002,
			MerchantName: "integration-test",
		},
	})
	require.NoError(t, err)
	require.NotZero(t, createProductResp.GetId())

	adjustResp, err := inventoryClient.AdjustStock(ctx, &inventoryv1.AdjustStockReq{
		Reason: "compose_e2e_timeout_seed",
		Items: []*inventoryv1.StockItem{{
			ProductId: createProductResp.GetId(),
			Quantity:  2,
		}},
	})
	require.NoError(t, err)
	require.EqualValues(t, 0, adjustResp.GetStatusCode())

	placeResp, err := checkoutClient.PlaceOrder(ctx, &checkoutv1.PlaceOrderReq{
		UserId: userID,
		Items: []*checkoutv1.CheckoutItem{{
			ProductId: createProductResp.GetId(),
			Quantity:  1,
		}},
		Address: &checkoutv1.Address{
			ReceiverName: "timeout buyer",
			Phone:        "13800138001",
			Province:     "Shanghai",
			City:         "Shanghai",
			District:     "Minhang",
			Street:       "No.2 Timeout Road",
			ZipCode:      "200240",
		},
		PaymentMethod:  "WECHAT_NATIVE",
		Currency:       "CNY",
		ExpectedAmount: 399,
	})
	require.NoError(t, err)
	require.NotZero(t, placeResp.GetOrderId())

	orderResp, err := orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: placeResp.GetOrderId()})
	require.NoError(t, err)
	require.Equal(t, orderv1.OrderStatus_ORDER_STATUS_CREATED, orderResp.GetOrder().GetOrderStatus())

	batchResp, err := inventoryClient.BatchGetInventory(ctx, &inventoryv1.BatchGetInventoryReq{
		ProductIds: []int64{createProductResp.GetId()},
	})
	require.NoError(t, err)
	require.Len(t, batchResp.GetInventories(), 1)
	require.EqualValues(t, 1, batchResp.GetInventories()[0].GetAvailableStock())

	db := openMySQL(t)
	defer db.Close()
	rdb := openRedis(t)
	defer func() { _ = rdb.Close() }()

	_, err = db.ExecContext(context.Background(), "UPDATE orders SET expired_at = DATE_SUB(UTC_TIMESTAMP(), INTERVAL 2 MINUTE) WHERE id = ?", placeResp.GetOrderId())
	require.NoError(t, err)
	require.NoError(t, rdb.ZAdd(context.Background(), "order:timeout:queue", redis.Z{
		Score:  float64(time.Now().Add(-2 * time.Minute).UnixMilli()),
		Member: fmt.Sprintf("%d", placeResp.GetOrderId()),
	}).Err())

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		orderResp, err = orderClient.GetOrder(pollCtx, &orderv1.GetOrderReq{OrderId: placeResp.GetOrderId()})
		return err == nil && orderResp.GetOrder().GetOrderStatus() == orderv1.OrderStatus_ORDER_STATUS_CANCELED
	}, 90*time.Second, time.Second, "checkout order should be canceled by timeout job")

	require.Eventually(t, func() bool {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer pollCancel()

		batchResp, err = inventoryClient.BatchGetInventory(pollCtx, &inventoryv1.BatchGetInventoryReq{
			ProductIds: []int64{createProductResp.GetId()},
		})
		return err == nil &&
			len(batchResp.GetInventories()) == 1 &&
			batchResp.GetInventories()[0].GetAvailableStock() == 2
	}, 30*time.Second, 500*time.Millisecond, "stock should be restored after timeout cancellation")
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
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.PingContext(ctx) == nil
	}, 15*time.Second, 500*time.Millisecond, "mysql is not reachable")

	return db
}

func openRedis(t *testing.T) *redis.Client {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return rdb.Ping(ctx).Err() == nil
	}, 15*time.Second, 500*time.Millisecond, "redis is not reachable")

	return rdb
}
