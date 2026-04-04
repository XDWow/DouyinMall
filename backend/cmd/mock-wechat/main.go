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

	log.Println("mock wechat pay server listening on http://localhost:8888")
	if err := router.Run(":8888"); err != nil {
		log.Fatal(err)
	}
}

func setupRouter(server *MockWechatPayServer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	v3 := router.Group("/v3")
	{
		pay := v3.Group("/pay/transactions")
		pay.POST("/native", server.Prepay)
		pay.GET("/out-trade-no/:outTradeNo", server.QueryOrder)
	}

	mock := router.Group("/mock")
	{
		mock.POST("/pay/:outTradeNo", server.MockPaySuccess)
		mock.GET("/orders", server.ListOrders)
	}

	return router
}

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
		c.JSON(http.StatusBadRequest, gin.H{"code": "PARAM_ERROR", "message": "invalid request"})
		return
	}

	transactionID := fmt.Sprintf("MOCK%d", time.Now().UnixNano())
	order := &MockOrder{
		OutTradeNo:    req.OutTradeNo,
		TransactionID: transactionID,
		TradeState:    "NOTPAY",
		Amount:        req.Amount.Total,
		Description:   req.Description,
		CreateTime:    time.Now(),
		NotifyURL:     req.NotifyURL,
	}

	s.mu.Lock()
	s.orders[req.OutTradeNo] = order
	s.mu.Unlock()

	log.Printf("mock prepay created: %s -> %s", req.OutTradeNo, transactionID)
	c.JSON(http.StatusOK, gin.H{
		"code_url": fmt.Sprintf("weixin://wxpay/bizpayurl?pr=MOCK_%s", req.OutTradeNo),
	})
}

func (s *MockWechatPayServer) QueryOrder(c *gin.Context) {
	outTradeNo := c.Param("outTradeNo")
	mchID := c.Query("mchid")

	s.mu.RLock()
	order, exists := s.orders[outTradeNo]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "ORDER_NOT_EXIST",
			"message": "order not found",
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

	c.JSON(http.StatusOK, resp)
}

func (s *MockWechatPayServer) MockPaySuccess(c *gin.Context) {
	outTradeNo := c.Param("outTradeNo")
	sendCallback := c.DefaultQuery("callback", "true") != "false"

	s.mu.Lock()
	order, exists := s.orders[outTradeNo]
	if !exists {
		s.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	now := time.Now()
	order.TradeState = "SUCCESS"
	order.SuccessTime = &now
	s.mu.Unlock()

	if sendCallback && order.NotifyURL != "" {
		go s.sendCallback(order)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "payment success",
		"out_trade_no":   order.OutTradeNo,
		"transaction_id": order.TransactionID,
		"callback_sent":  sendCallback,
	})
}

func (s *MockWechatPayServer) sendCallback(order *MockOrder) {
	callback := map[string]any{
		"trade_state":    order.TradeState,
		"transaction_id": order.TransactionID,
		"out_trade_no":   order.OutTradeNo,
	}

	body, _ := json.Marshal(callback)
	resp, err := http.Post(order.NotifyURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("mock callback send failed: %s -> %v", order.NotifyURL, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("mock callback delivered: %s -> %d", order.NotifyURL, resp.StatusCode)
}

func (s *MockWechatPayServer) ListOrders(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]*MockOrder, 0, len(s.orders))
	for _, order := range s.orders {
		orders = append(orders, order)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  len(orders),
		"orders": orders,
	})
}

func getTradeStateDesc(state string) string {
	switch state {
	case "SUCCESS":
		return "payment success"
	case "REFUND":
		return "refund"
	case "NOTPAY":
		return "not paid"
	case "CLOSED":
		return "closed"
	case "REVOKED":
		return "revoked"
	case "USERPAYING":
		return "user paying"
	case "PAYERROR":
		return "payment error"
	default:
		return "unknown"
	}
}


