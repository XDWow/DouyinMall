package http

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

type AlipayPageHandler struct {
	pagePayUC    *usecase.AlipayPagePayUC
	returnPageUC *usecase.AlipayReturnPageUC
	provider     string
	l            logger.LoggerV1
}

func NewAlipayPageHandler(
	pagePayUC *usecase.AlipayPagePayUC,
	returnPageUC *usecase.AlipayReturnPageUC,
	provider string,
	l logger.LoggerV1,
) *AlipayPageHandler {
	return &AlipayPageHandler{
		pagePayUC:    pagePayUC,
		returnPageUC: returnPageUC,
		provider:     strings.ToLower(strings.TrimSpace(provider)),
		l:            l,
	}
}

type alipayPageRequest struct {
	OrderID int64  `form:"order_id" json:"order_id"`
	Mode    string `form:"mode" json:"mode"`
}

func (h *AlipayPageHandler) HandlePagePay(c *gin.Context) {
	if h.provider != providerAlipay || h.pagePayUC == nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "alipay page pay is not enabled"})
		return
	}

	var req alipayPageRequest
	var err error
	switch c.Request.Method {
	case http.MethodPost:
		if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
			err = c.ShouldBindJSON(&req)
		} else {
			err = c.ShouldBind(&req)
		}
	default:
		err = c.ShouldBindQuery(&req)
	}
	if err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "order_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	result, err := h.pagePayUC.Execute(ctx, usecase.AlipayPagePayCmd{OrderID: req.OrderID})
	if err != nil {
		h.writePagePayError(c, err)
		return
	}

	if c.Request.Method == http.MethodPost || strings.EqualFold(req.Mode, "url") {
		c.JSON(http.StatusOK, gin.H{
			"order_id":     result.OrderID,
			"out_trade_no": result.OutTradeNo,
			"total_amount": result.TotalAmount,
			"subject":      result.Subject,
			"payment_url":  result.PaymentURL,
		})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buildAutoRedirectHTML(result.PaymentURL))
}

func (h *AlipayPageHandler) HandleSuccessPage(c *gin.Context) {
	orderID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("order_id")), 10, 64)
	outTradeNo := strings.TrimSpace(c.Query("out_trade_no"))

	var result *usecase.AlipayReturnPageResult
	if h.returnPageUC != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		loaded, err := h.returnPageUC.Execute(ctx, usecase.AlipayReturnPageCmd{
			OrderID:    orderID,
			OutTradeNo: outTradeNo,
		})
		if err != nil {
			h.l.Warn("查询支付成功页订单状态失败", logger.Error(err))
		} else {
			result = loaded
		}
	}
	if result == nil {
		result = &usecase.AlipayReturnPageResult{
			OrderID:    orderID,
			OutTradeNo: outTradeNo,
		}
	}

	// 这里只是同步跳转展示页，不做最终支付确认
	// 最终支付结果以后端 notify_url 异步通知为准
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, renderSuccessPage(result))
}

func (h *AlipayPageHandler) RegisterRoutes(router *gin.Engine) {
	apiGroup := router.Group("/api/pay/alipay")
	apiGroup.GET("/page", h.HandlePagePay)
	apiGroup.POST("/page", h.HandlePagePay)

	router.GET("/pay/success", h.HandleSuccessPage)
}

func (h *AlipayPageHandler) writePagePayError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case err == nil:
		status = http.StatusOK
	case strings.Contains(err.Error(), "query order failed"):
		status = http.StatusBadGateway
	case err == domain.ErrPaymentAlreadyPaid:
		status = http.StatusConflict
	case err == domain.ErrPaymentAmountChanged:
		status = http.StatusConflict
	case err == domain.ErrOrderNotPayable:
		status = http.StatusConflict
	case err == domain.ErrPagePayNotEnabled:
		status = http.StatusBadRequest
	default:
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"message": err.Error()})
}

func buildAutoRedirectHTML(paymentURL string) string {
	escaped := template.HTMLEscapeString(paymentURL)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>跳转支付宝收银台</title>
  <meta http-equiv="refresh" content="0;url=%s">
</head>
<body>
  <p>正在跳转到支付宝收银台...</p>
  <p>如果没有自动跳转，请点击这里：<a href="%s">立即支付</a></p>
</body>
</html>`, escaped, escaped)
}

func renderSuccessPage(result *usecase.AlipayReturnPageResult) string {
	orderID := "-"
	if result.OrderID > 0 {
		orderID = strconv.FormatInt(result.OrderID, 10)
	}
	outTradeNo := result.OutTradeNo
	if outTradeNo == "" {
		outTradeNo = "-"
	}
	orderStatus := result.OrderStatus
	if orderStatus == "" {
		orderStatus = "UNKNOWN"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>支付处理中</title>
</head>
<body>
  <h1>支付结果请以后端异步通知为准</h1>
  <p>当前页面只是支付宝完成支付后的同步跳转页，不代表订单一定已经支付成功。</p>
  <p>订单号：%s</p>
  <p>商户单号：%s</p>
  <p>当前订单状态：%s</p>
</body>
</html>`,
		template.HTMLEscapeString(orderID),
		template.HTMLEscapeString(outTradeNo),
		template.HTMLEscapeString(orderStatus),
	)
}
