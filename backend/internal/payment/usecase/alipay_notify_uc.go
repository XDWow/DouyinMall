package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type AlipayNotifyCmd struct {
	AppID       string
	SellerID    string
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	TotalAmount string
}

type AlipayNotifyUC struct {
	repo       domain.PaymentRepository
	callbackUC *PayCallbackUC
	cfg        AlipayWebConfig
}

func NewAlipayNotifyUC(
	repo domain.PaymentRepository,
	callbackUC *PayCallbackUC,
	cfg AlipayWebConfig,
) *AlipayNotifyUC {
	return &AlipayNotifyUC{
		repo:       repo,
		callbackUC: callbackUC,
		cfg:        cfg.Normalize(),
	}
}

func (uc *AlipayNotifyUC) Execute(ctx context.Context, cmd AlipayNotifyCmd) error {
	cmd.AppID = strings.TrimSpace(cmd.AppID)
	cmd.SellerID = strings.TrimSpace(cmd.SellerID)
	cmd.OutTradeNo = strings.TrimSpace(cmd.OutTradeNo)
	cmd.TradeNo = strings.TrimSpace(cmd.TradeNo)
	cmd.TradeStatus = strings.TrimSpace(cmd.TradeStatus)
	cmd.TotalAmount = strings.TrimSpace(cmd.TotalAmount)

	if cmd.AppID == "" || cmd.OutTradeNo == "" || cmd.TradeStatus == "" || cmd.TotalAmount == "" {
		return domain.ErrInvalidNotifyData
	}
	if uc.cfg.AppID == "" || cmd.AppID != uc.cfg.AppID {
		return fmt.Errorf("%w: app_id mismatch", domain.ErrInvalidNotifyData)
	}
	if uc.cfg.PID != "" && cmd.SellerID != "" && cmd.SellerID != uc.cfg.PID {
		return fmt.Errorf("%w: seller_id mismatch", domain.ErrInvalidNotifyData)
	}
	if !isSupportedTradeStatus(cmd.TradeStatus) {
		return fmt.Errorf("%w: unsupported trade_status=%s", domain.ErrInvalidNotifyData, cmd.TradeStatus)
	}

	pmt, err := uc.repo.GetPayment(ctx, cmd.OutTradeNo)
	if err != nil {
		return err
	}

	amount, err := yuanStringToCents(cmd.TotalAmount)
	if err != nil {
		return fmt.Errorf("%w: invalid total_amount", domain.ErrInvalidNotifyData)
	}
	if amount != pmt.Amt.Total {
		return domain.ErrPaymentAmountChanged
	}

	// 支付回调必须做幂等。
	// 支付宝会因为网络、超时等原因重复通知，已成功的订单再收到通知时直接返回 success。
	if pmt.Status == domain.PaymentStatusSuccess {
		return nil
	}

	if isSuccessTradeStatus(cmd.TradeStatus) && cmd.TradeNo == "" {
		return domain.ErrPaymentTxnIDRequired
	}

	return uc.callbackUC.Execute(ctx, CallbackCmd{
		TradeState:    cmd.TradeStatus,
		TransactionId: cmd.TradeNo,
		OutTradeNo:    cmd.OutTradeNo,
	})
}

func isSupportedTradeStatus(status string) bool {
	switch status {
	case "WAIT_BUYER_PAY", "TRADE_SUCCESS", "TRADE_FINISHED", "TRADE_CLOSED":
		return true
	default:
		return false
	}
}

func isSuccessTradeStatus(status string) bool {
	return status == "TRADE_SUCCESS" || status == "TRADE_FINISHED"
}

func yuanStringToCents(amount string) (int64, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return 0, errors.New("empty amount")
	}
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0, err
	}
	return int64(f*100 + 0.5), nil
}
