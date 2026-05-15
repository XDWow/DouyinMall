package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

type AlipayPagePayUC struct {
	orderClient orderservice.Client
	repo        domain.PaymentRepository
	pageSvc     domain.PagePayService
	cfg         AlipayWebConfig
	l           logger.LoggerV1
}

type AlipayPagePayCmd struct {
	OrderID int64
}

type AlipayPagePayResult struct {
	OrderID     int64
	OutTradeNo  string
	PaymentURL  string
	TotalAmount string
	Subject     string
}

func NewAlipayPagePayUC(
	orderClient orderservice.Client,
	repo domain.PaymentRepository,
	pageSvc domain.PagePayService,
	cfg AlipayWebConfig,
	l logger.LoggerV1,
) *AlipayPagePayUC {
	return &AlipayPagePayUC{
		orderClient: orderClient,
		repo:        repo,
		pageSvc:     pageSvc,
		cfg:         cfg.Normalize(),
		l:           l,
	}
}

func (uc *AlipayPagePayUC) Execute(ctx context.Context, cmd AlipayPagePayCmd) (*AlipayPagePayResult, error) {
	if cmd.OrderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	if uc.pageSvc == nil {
		return nil, domain.ErrPagePayNotEnabled
	}
	if uc.orderClient == nil {
		return nil, fmt.Errorf("order client is nil")
	}
	if uc.cfg.NotifyURL == "" || uc.cfg.ReturnURL == "" {
		return nil, fmt.Errorf("alipay notify_url or return_url is empty")
	}

	orderResp, err := uc.orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: cmd.OrderID})
	if err != nil {
		return nil, fmt.Errorf("query order failed: %w", err)
	}

	orderInfo := orderResp.GetOrder()
	if orderInfo == nil {
		return nil, domain.ErrOrderNotPayable
	}
	if orderInfo.GetOrderStatus() == orderv1.OrderStatus_ORDER_STATUS_PAID {
		return nil, domain.ErrPaymentAlreadyPaid
	}
	if orderInfo.GetOrderStatus() != orderv1.OrderStatus_ORDER_STATUS_CREATED {
		return nil, domain.ErrOrderNotPayable
	}
	if orderInfo.GetTotalAmount() <= 0 {
		return nil, fmt.Errorf("order payable amount must be positive")
	}

	outTradeNo := strconv.FormatInt(orderInfo.GetOrderId(), 10)
	subject := buildPagePaySubject(orderInfo.GetOrderId(), orderInfo.GetOrderKind())
	payment := domain.Payment{
		BizTradeNo:  outTradeNo,
		Description: subject,
		Amt: domain.Amount{
			Currency: defaultCurrency(orderInfo.GetCurrency()),
			Total:    orderInfo.GetTotalAmount(),
		},
	}

	if err = uc.ensurePaymentRecord(ctx, payment); err != nil {
		return nil, err
	}

	totalAmount := centsToYuan(orderInfo.GetTotalAmount())
	paymentURL, err := uc.pageSvc.BuildPagePayURL(ctx, domain.PagePayRequest{
		OutTradeNo:  outTradeNo,
		Subject:     subject,
		TotalAmount: totalAmount,
		SellerID:    uc.cfg.PID,
		NotifyURL:   uc.cfg.NotifyURL,
		ReturnURL:   uc.cfg.ReturnURL,
	})
	if err != nil {
		uc.l.Error("生成支付宝电脑网站支付链接失败",
			logger.Error(err),
			logger.Int64("orderID", orderInfo.GetOrderId()))
		return nil, err
	}

	return &AlipayPagePayResult{
		OrderID:     orderInfo.GetOrderId(),
		OutTradeNo:  outTradeNo,
		PaymentURL:  paymentURL,
		TotalAmount: totalAmount,
		Subject:     subject,
	}, nil
}

func (uc *AlipayPagePayUC) ensurePaymentRecord(ctx context.Context, pmt domain.Payment) error {
	existing, err := uc.repo.GetPayment(ctx, pmt.BizTradeNo)
	switch {
	case err == nil:
		if existing.Status == domain.PaymentStatusSuccess {
			return domain.ErrPaymentAlreadyPaid
		}
		if existing.Amt.Total != pmt.Amt.Total || existing.Amt.Currency != pmt.Amt.Currency {
			return domain.ErrPaymentAmountChanged
		}
		return nil
	case errors.Is(err, domain.ErrPaymentNotFound):
		return uc.repo.AddPayment(ctx, pmt)
	default:
		return err
	}
}

func buildPagePaySubject(orderID int64, orderKind string) string {
	orderKind = strings.TrimSpace(orderKind)
	if orderKind == "" {
		return fmt.Sprintf("DouyinMall 订单 %d", orderID)
	}
	return fmt.Sprintf("DouyinMall %s 订单 %d", orderKind, orderID)
}

func defaultCurrency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return "CNY"
	}
	return currency
}

func centsToYuan(total int64) string {
	sign := ""
	if total < 0 {
		sign = "-"
		total = -total
	}
	return fmt.Sprintf("%s%d.%02d", sign, total/100, total%100)
}
