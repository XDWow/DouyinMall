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
	alipayNotifyUC     *usecase.AlipayNotifyUC
	wechatNotifyHandle *notify.Handler
	alipayClient       *ali.Client
	provider           string
	l                  logger.LoggerV1
}

func NewPaymentCallbackHandler(
	payCallbackUC *usecase.PayCallbackUC,
	alipayNotifyUC *usecase.AlipayNotifyUC,
	wechatNotifyHandle *notify.Handler,
	alipayClient *ali.Client,
	provider string,
	l logger.LoggerV1,
) *PaymentCallbackHandler {
	return &PaymentCallbackHandler{
		payCallbackUC:      payCallbackUC,
		alipayNotifyUC:     alipayNotifyUC,
		wechatNotifyHandle: wechatNotifyHandle,
		alipayClient:       alipayClient,
		provider:           strings.ToLower(strings.TrimSpace(provider)),
		l:                  l,
	}
}

func (h *PaymentCallbackHandler) HandleWechatCallback(c *gin.Context) {
	if h.provider != providerMockWechat && h.provider != providerWechat {
		h.respondWechatFail(c, "wechat callback is not enabled")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.l.Error("读取微信回调请求体失败", logger.Error(err))
		h.respondWechatFail(c, "read request body failed")
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
		h.l.Error("解析微信回调失败", logger.Error(err), logger.String("body", string(body)))
		h.respondWechatFail(c, "invalid callback payload")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err = h.payCallbackUC.Execute(ctx, cmd); err != nil {
		h.l.Error("处理微信回调失败",
			logger.Error(err),
			logger.String("out_trade_no", cmd.OutTradeNo))
		h.respondWechatFail(c, "process callback failed")
		return
	}

	h.respondWechatSuccess(c)
}

func (h *PaymentCallbackHandler) HandleAlipayCallback(c *gin.Context) {
	if h.provider != providerAlipay || h.alipayClient == nil || h.alipayNotifyUC == nil {
		h.respondAlipayFail(c)
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		h.l.Error("解析支付宝回调表单失败", logger.Error(err))
		h.respondAlipayFail(c)
		return
	}

	notification, err := h.alipayClient.DecodeNotification(c.Request.Context(), c.Request.PostForm)
	if err != nil {
		// DecodeNotification 内部已经完成验签。
		h.l.Error("验签或解码支付宝回调失败", logger.Error(err))
		h.respondAlipayFail(c)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// return_url 只是浏览器跳转页，不能作为最终支付成功依据。
	// 最终状态只认 notify_url 异步通知，因为它才是服务端可靠收到的结果。
	if err = h.alipayNotifyUC.Execute(ctx, usecase.AlipayNotifyCmd{
		AppID:       notification.AppId,
		SellerID:    notification.SellerId,
		OutTradeNo:  notification.OutTradeNo,
		TradeNo:     notification.TradeNo,
		TradeStatus: string(notification.TradeStatus),
		TotalAmount: notification.TotalAmount,
	}); err != nil {
		h.l.Error("处理支付宝异步通知失败",
			logger.Error(err),
			logger.String("out_trade_no", notification.OutTradeNo),
			logger.String("trade_status", string(notification.TradeStatus)))
		h.respondAlipayFail(c)
		return
	}

	// 支付宝收到 success 后才会停止重试，所以这里必须返回纯文本 success。
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
	if transaction.TradeState == nil || transaction.OutTradeNo == nil {
		return usecase.CallbackCmd{}, fmt.Errorf("wechat callback missing required fields")
	}

	transactionID := ""
	if transaction.TransactionId != nil {
		transactionID = *transaction.TransactionId
	}

	return usecase.CallbackCmd{
		TradeState:    string(*transaction.TradeState),
		TransactionId: transactionID,
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
		TradeState:    strings.TrimSpace(payload.TradeState),
		TransactionId: strings.TrimSpace(payload.TransactionID),
		OutTradeNo:    strings.TrimSpace(payload.OutTradeNo),
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

	apiPayGroup := router.Group("/api/pay")
	apiPayGroup.POST("/alipay/notify", h.HandleAlipayCallback)
}
