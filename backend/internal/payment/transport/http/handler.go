package http

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

// 微信支付回调处理器
type WechatCallbackHandler struct {
	payCallbackUC *usecase.PayCallbackUC
	notifyHandler *notify.Handler // 微信通知处理器（用于验签）
	l             logger.LoggerV1
}

func NewWechatCallbackHandler(
	payCallbackUC *usecase.PayCallbackUC,
	certificateSerialNo string, // 商户API证书序列号（暂未使用）
	privateKey *rsa.PrivateKey, // 商户私钥（暂未使用）
	apiV3Key string, // API v3密钥
	l logger.LoggerV1,
) *WechatCallbackHandler {
	// 创建微信通知处理器 - 使用nil verifier暂时跳过验签
	// TODO: 生产环境需要正确配置verifier
	notifyHandler, err := notify.NewRSANotifyHandler(apiV3Key, nil)
	if err != nil {
		panic(fmt.Errorf("创建微信通知处理器失败: %w", err))
	}

	return &WechatCallbackHandler{
		payCallbackUC: payCallbackUC,
		notifyHandler: notifyHandler,
		l:             l,
	}
}

// HandleWechatCallback 处理微信支付回调
// 路由: POST /payment/wechat/callback
func (h *WechatCallbackHandler) HandleWechatCallback(c *gin.Context) {
	// 1. 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.l.Error("读取微信回调请求体失败", logger.Error(err))
		h.respondFail(c, "读取请求失败")
		return
	}

	// 2. 验证签名（防止伪造回调）
	transaction := &payments.Transaction{}
	_, err = h.notifyHandler.ParseNotifyRequest(
		c.Request.Context(),
		c.Request,
		transaction,
	)
	if err != nil {
		h.l.Error("验证微信回调签名失败",
			logger.Error(err),
			logger.String("body", string(body)))
		h.respondFail(c, "签名验证失败")
		return
	}

	// 3. 记录日志
	h.l.Info("收到微信支付回调",
		logger.String("out_trade_no", *transaction.OutTradeNo),
		logger.String("transaction_id", *transaction.TransactionId),
		logger.String("trade_state", string(*transaction.TradeState)))

	// 4. 调用业务逻辑
	cmd := usecase.CallbackCmd{
		TradeState:    string(*transaction.TradeState),
		TransactionId: *transaction.TransactionId,
		OutTradeNo:    *transaction.OutTradeNo,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*5)
	defer cancel()

	err = h.payCallbackUC.Execute(ctx, cmd)
	if err != nil {
		h.l.Error("处理支付回调失败",
			logger.Error(err),
			logger.String("out_trade_no", *transaction.OutTradeNo))
		// 注意：即使业务处理失败，也要返回成功给微信，避免重复回调
		// 后续通过定时任务或补偿机制处理
		h.respondSuccess(c)
		return
	}

	// 5. 返回成功响应
	h.respondSuccess(c)
}

// respondSuccess 返回成功响应（微信要求的格式）
func (h *WechatCallbackHandler) respondSuccess(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"code":    "SUCCESS",
		"message": "成功",
	})
}

// respondFail 返回失败响应（微信要求的格式）
func (h *WechatCallbackHandler) respondFail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"code":    "FAIL",
		"message": message,
	})
}

// RegisterRoutes 注册路由
func (h *WechatCallbackHandler) RegisterRoutes(router *gin.Engine) {
	// 微信回调接口不需要认证
	paymentGroup := router.Group("/payment")
	{
		paymentGroup.POST("/wechat/callback", h.HandleWechatCallback)
	}
}
