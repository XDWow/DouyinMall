package http

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

type WechatCallbackHandler struct {
	payCallbackUC *usecase.PayCallbackUC
	notifyHandler *notify.Handler
	l             logger.LoggerV1
	mockMode      bool
}

func NewWechatCallbackHandler(
	payCallbackUC *usecase.PayCallbackUC,
	_ string,
	_ *rsa.PrivateKey,
	apiV3Key string,
	l logger.LoggerV1,
) *WechatCallbackHandler {
	handler := &WechatCallbackHandler{
		payCallbackUC: payCallbackUC,
		l:             l,
		mockMode:      strings.EqualFold(viper.GetString("payment.mode"), "mock"),
	}
	if handler.mockMode {
		return handler
	}

	notifyHandler, err := notify.NewRSANotifyHandler(apiV3Key, nil)
	if err != nil {
		panic(fmt.Errorf("create wechat notify handler failed: %w", err))
	}
	handler.notifyHandler = notifyHandler
	return handler
}

func (h *WechatCallbackHandler) HandleWechatCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.l.Error("read wechat callback body failed", logger.Error(err))
		h.respondFail(c, "read request failed")
		return
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	var cmd usecase.CallbackCmd
	if h.mockMode {
		cmd, err = h.parseMockCallback(body)
	} else {
		cmd, err = h.parseRealCallback(c)
	}
	if err != nil {
		h.l.Error("parse wechat callback failed",
			logger.Error(err),
			logger.String("body", string(body)))
		h.respondFail(c, "invalid callback")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err = h.payCallbackUC.Execute(ctx, cmd); err != nil {
		h.l.Error("handle payment callback failed",
			logger.Error(err),
			logger.String("out_trade_no", cmd.OutTradeNo))
		h.respondFail(c, "handle callback failed")
		return
	}

	h.respondSuccess(c)
}

func (h *WechatCallbackHandler) parseRealCallback(c *gin.Context) (usecase.CallbackCmd, error) {
	transaction := &payments.Transaction{}
	_, err := h.notifyHandler.ParseNotifyRequest(
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

func (h *WechatCallbackHandler) parseMockCallback(body []byte) (usecase.CallbackCmd, error) {
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

func (h *WechatCallbackHandler) respondSuccess(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"code":    "SUCCESS",
		"message": "success",
	})
}

func (h *WechatCallbackHandler) respondFail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, map[string]any{
		"code":    "FAIL",
		"message": message,
	})
}

func (h *WechatCallbackHandler) RegisterRoutes(router *gin.Engine) {
	paymentGroup := router.Group("/payment")
	paymentGroup.POST("/wechat/callback", h.HandleWechatCallback)
}


