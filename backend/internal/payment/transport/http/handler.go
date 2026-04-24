package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	ali "github.com/smartwalle/alipay/v3"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

const (
	providerMockWechat = "mock_wechat"
	providerWechat     = "wechat"
	providerAlipay     = "alipay"
)

type PaymentCallbackHandler struct {
	payCallbackUC      *usecase.PayCallbackUC
	wechatNotifyHandle *notify.Handler
	alipayClient       *ali.Client
	provider           string
	l                  logger.LoggerV1
}

func NewPaymentCallbackHandler(
	payCallbackUC *usecase.PayCallbackUC,
	wechatNotifyHandle *notify.Handler,
	alipayClient *ali.Client,
	provider string,
	l logger.LoggerV1,
) *PaymentCallbackHandler {
	return &PaymentCallbackHandler{
		payCallbackUC:      payCallbackUC,
		wechatNotifyHandle: wechatNotifyHandle,
		alipayClient:       alipayClient,
		provider:           strings.ToLower(strings.TrimSpace(provider)),
		l:                  l,
	}
}

func (h *PaymentCallbackHandler) HandleWechatCallback(c *gin.Context) {
	if h.provider != providerMockWechat && h.provider != providerWechat {
		h.respondWechatFail(c, "wechat callback is disabled")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.l.Error("read wechat callback body failed", logger.Error(err))
		h.respondWechatFail(c, "read request failed")
		return
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	var cmd usecase.CallbackCmd
	if h.provider == providerMockWechat {
		cmd, err = h.parseMockWechatCallback(body)
	} else {
		cmd, err = h.parseWechatCallback(c)
	}
	if err != nil {
		h.l.Error("parse wechat callback failed",
			logger.Error(err),
			logger.String("body", string(body)))
		h.respondWechatFail(c, "invalid callback")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err = h.payCallbackUC.Execute(ctx, cmd); err != nil {
		h.l.Error("handle wechat callback failed",
			logger.Error(err),
			logger.String("out_trade_no", cmd.OutTradeNo))
		h.respondWechatFail(c, "handle callback failed")
		return
	}

	h.respondWechatSuccess(c)
}

func (h *PaymentCallbackHandler) HandleAlipayCallback(c *gin.Context) {
	if h.provider != providerAlipay || h.alipayClient == nil {
		h.respondAlipayFail(c)
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		h.l.Error("parse alipay callback form failed", logger.Error(err))
		h.respondAlipayFail(c)
		return
	}

	notification, err := h.alipayClient.DecodeNotification(c.Request.Context(), c.Request.PostForm)
	if err != nil {
		h.l.Error("decode alipay callback failed", logger.Error(err))
		h.respondAlipayFail(c)
		return
	}

	cmd := usecase.CallbackCmd{
		TradeState:    string(notification.TradeStatus),
		TransactionId: notification.TradeNo,
		OutTradeNo:    notification.OutTradeNo,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err = h.payCallbackUC.Execute(ctx, cmd); err != nil {
		h.l.Error("handle alipay callback failed",
			logger.Error(err),
			logger.String("out_trade_no", cmd.OutTradeNo))
		h.respondAlipayFail(c)
		return
	}

	h.respondAlipaySuccess(c)
}

func (h *PaymentCallbackHandler) parseWechatCallback(c *gin.Context) (usecase.CallbackCmd, error) {
	if h.wechatNotifyHandle == nil {
		return usecase.CallbackCmd{}, fmt.Errorf("wechat notify handler is not initialized")
	}

	transaction := &payments.Transaction{}
	_, err := h.wechatNotifyHandle.ParseNotifyRequest(
		c.Request.Context(),
		c.Request,
		transaction,
	)
	if err != nil {
		return usecase.CallbackCmd{}, err
	}

	return usecase.CallbackCmd{
		TradeState:    string(*transaction.TradeState),
		TransactionId: *transaction.TransactionId,
		OutTradeNo:    *transaction.OutTradeNo,
	}, nil
}

func (h *PaymentCallbackHandler) parseMockWechatCallback(body []byte) (usecase.CallbackCmd, error) {
	var payload struct {
		TradeState    string `json:"trade_state"`
		TransactionID string `json:"transaction_id"`
		OutTradeNo    string `json:"out_trade_no"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return usecase.CallbackCmd{}, err
	}
	if payload.OutTradeNo == "" || payload.TradeState == "" {
		return usecase.CallbackCmd{}, fmt.Errorf("mock callback missing required fields")
	}
	return usecase.CallbackCmd{
		TradeState:    payload.TradeState,
		TransactionId: payload.TransactionID,
		OutTradeNo:    payload.OutTradeNo,
	}, nil
}

func (h *PaymentCallbackHandler) respondWechatSuccess(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"code":    "SUCCESS",
		"message": "success",
	})
}

func (h *PaymentCallbackHandler) respondWechatFail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, map[string]any{
		"code":    "FAIL",
		"message": message,
	})
}

func (h *PaymentCallbackHandler) respondAlipaySuccess(c *gin.Context) {
	c.String(http.StatusOK, "success")
}

func (h *PaymentCallbackHandler) respondAlipayFail(c *gin.Context) {
	c.String(http.StatusOK, "fail")
}

func (h *PaymentCallbackHandler) RegisterRoutes(router *gin.Engine) {
	paymentGroup := router.Group("/payment")
	paymentGroup.POST("/wechat/callback", h.HandleWechatCallback)
	paymentGroup.POST("/alipay/callback", h.HandleAlipayCallback)
}
