package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MockWechatPayServer 模拟微信支付服务端
// 提供和微信支付 API v3 相同的接口
type MockWechatPayServer struct {
	orders map[string]*MockOrder
	mu     sync.RWMutex
}

type MockOrder struct {
	OutTradeNo    string     `json:"out_trade_no"`
	TransactionID string     `json:"transaction_id"`
	TradeState    string     `json:"trade_state"`
	Amount        int64      `json:"amount"`
	Description   string     `json:"description"`
	CreateTime    time.Time  `json:"create_time"`
	SuccessTime   *time.Time `json:"success_time,omitempty"`
	NotifyURL     string     `json:"-"`
}

func NewMockWechatPayServer() *MockWechatPayServer {
	return &MockWechatPayServer{
		orders: make(map[string]*MockOrder),
	}
}

func main() {
	server := NewMockWechatPayServer()
	router := setupRouter(server)

	log.Println("🚀 Mock 微信支付服务启动在 http://localhost:8888")
	log.Println("📝 提供以下接口:")
	log.Println("   POST /v3/pay/transactions/native - 预支付")
	log.Println("   GET  /v3/pay/transactions/out-trade-no/:outTradeNo - 查询订单")
	log.Println("   POST /mock/pay/:outTradeNo - 模拟支付成功（测试用）")

	if err := router.Run(":8888"); err != nil {
		log.Fatal(err)
	}
}

func setupRouter(server *MockWechatPayServer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 微信支付 API v3 接口
	v3 := router.Group("/v3")
	{
		pay := v3.Group("/pay/transactions")
		{
			// 预支付接口
			pay.POST("/native", server.Prepay)
			// 查询订单接口
			pay.GET("/out-trade-no/:outTradeNo", server.QueryOrder)
		}
	}

	// Mock 测试接口（方便测试）
	mock := router.Group("/mock")
	{
		// 模拟支付成功
		mock.POST("/pay/:outTradeNo", server.MockPaySuccess)
		// 查看所有订单
		mock.GET("/orders", server.ListOrders)
	}

	return router
}

// Prepay 预支付接口（模拟微信的 Native 下单 API）
func (s *MockWechatPayServer) Prepay(c *gin.Context) {
	var req struct {
		AppID       string `json:"appid"`
		MchID       string `json:"mchid"`
		Description string `json:"description"`
		OutTradeNo  string `json:"out_trade_no"`
		NotifyURL   string `json:"notify_url"`
		Amount      struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "PARAM_ERROR", "message": "参数错误"})
		return
	}

	// 生成模拟的交易单号
	transactionID := fmt.Sprintf("MOCK%d", time.Now().UnixNano())

	// 保存订单
	order := &MockOrder{
		OutTradeNo:    req.OutTradeNo,
		TransactionID: transactionID,
		TradeState:    "NOTPAY", // 未支付
		Amount:        req.Amount.Total,
		Description:   req.Description,
		CreateTime:    time.Now(),
		NotifyURL:     req.NotifyURL,
	}

	s.mu.Lock()
	s.orders[req.OutTradeNo] = order
	s.mu.Unlock()

	log.Printf("✅ 预支付订单创建: %s -> %s", req.OutTradeNo, transactionID)

	// 返回二维码链接（模拟）
	codeURL := fmt.Sprintf("weixin://wxpay/bizpayurl?pr=MOCK_%s", req.OutTradeNo)
	c.JSON(200, gin.H{
		"code_url": codeURL,
	})
}

// QueryOrder 查询订单接口（模拟微信的查询订单 API）
func (s *MockWechatPayServer) QueryOrder(c *gin.Context) {
	outTradeNo := c.Param("outTradeNo")
	mchID := c.Query("mchid")

	s.mu.RLock()
	order, exists := s.orders[outTradeNo]
	s.mu.RUnlock()

	if !exists {
		c.JSON(404, gin.H{
			"code":    "ORDER_NOT_EXIST",
			"message": "订单不存在",
		})
		return
	}

	resp := gin.H{
		"out_trade_no":     order.OutTradeNo,
		"transaction_id":   order.TransactionID,
		"trade_state":      order.TradeState,
		"trade_state_desc": getTradeStateDesc(order.TradeState),
		"amount": gin.H{
			"total":    order.Amount,
			"currency": "CNY",
		},
		"mchid": mchID,
	}

	if order.SuccessTime != nil {
		resp["success_time"] = order.SuccessTime.Format(time.RFC3339)
	}

	log.Printf("📋 查询订单: %s -> %s", outTradeNo, order.TradeState)

	c.JSON(200, resp)
}

// MockPaySuccess 模拟支付成功（测试用接口）
func (s *MockWechatPayServer) MockPaySuccess(c *gin.Context) {
	outTradeNo := c.Param("outTradeNo")

	s.mu.Lock()
	order, exists := s.orders[outTradeNo]
	if !exists {
		s.mu.Unlock()
		c.JSON(404, gin.H{"error": "订单不存在"})
		return
	}

	// 更新订单状态为成功
	now := time.Now()
	order.TradeState = "SUCCESS"
	order.SuccessTime = &now
	s.mu.Unlock()

	log.Printf("💰 模拟支付成功: %s", outTradeNo)

	// 发送回调通知（如果有配置回调地址）
	if order.NotifyURL != "" {
		go s.sendCallback(order)
	}

	c.JSON(200, gin.H{
		"message":        "支付成功",
		"out_trade_no":   order.OutTradeNo,
		"transaction_id": order.TransactionID,
	})
}

// sendCallback 发送支付回调通知到商户服务器
func (s *MockWechatPayServer) sendCallback(order *MockOrder) {
	// 构造回调数据（简化版，真实微信会加密）
	callback := map[string]interface{}{
		"id":            fmt.Sprintf("mock_callback_%d", time.Now().Unix()),
		"create_time":   time.Now().Format(time.RFC3339),
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"summary":       "支付成功",
		"resource": map[string]interface{}{
			"algorithm":  "AEAD_AES_256_GCM",
			"ciphertext": "", // Mock 环境可以不加密
			"nonce":      "",
		},
	}

	body, _ := json.Marshal(callback)
	resp, err := http.Post(order.NotifyURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("⚠️  回调发送失败: %s -> %v", order.NotifyURL, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("🔔 回调已发送: %s -> %d", order.NotifyURL, resp.StatusCode)
}

// ListOrders 查看所有订单（测试用）
func (s *MockWechatPayServer) ListOrders(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]*MockOrder, 0, len(s.orders))
	for _, order := range s.orders {
		orders = append(orders, order)
	}

	c.JSON(200, gin.H{
		"total":  len(orders),
		"orders": orders,
	})
}

func getTradeStateDesc(state string) string {
	switch state {
	case "SUCCESS":
		return "支付成功"
	case "REFUND":
		return "转入退款"
	case "NOTPAY":
		return "未支付"
	case "CLOSED":
		return "已关闭"
	case "REVOKED":
		return "已撤销（付款码支付）"
	case "USERPAYING":
		return "用户支付中（付款码支付）"
	case "PAYERROR":
		return "支付失败（付款码支付）"
	default:
		return "未知状态"
	}
}
