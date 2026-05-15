package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type notifyTestRepo struct {
	payment domain.Payment
	err     error
}

func (r *notifyTestRepo) AddPayment(context.Context, domain.Payment) error {
	return nil
}

func (r *notifyTestRepo) UpdatePayment(context.Context, domain.Payment) error {
	return nil
}

func (r *notifyTestRepo) ApplyProviderResult(context.Context, domain.Payment) (domain.Payment, bool, error) {
	return domain.Payment{}, false, nil
}

func (r *notifyTestRepo) GetPayment(context.Context, string) (domain.Payment, error) {
	if r.err != nil {
		return domain.Payment{}, r.err
	}
	return r.payment, nil
}

func (r *notifyTestRepo) FindExpiredPayment(context.Context, int, time.Time) ([]domain.Payment, error) {
	return nil, nil
}

func TestAlipayNotifyUC_IdempotentWhenAlreadyPaid(t *testing.T) {
	repo := &notifyTestRepo{
		payment: domain.Payment{
			BizTradeNo: "1001",
			Status:     domain.PaymentStatusSuccess,
			Amt: domain.Amount{
				Currency: "CNY",
				Total:    9900,
			},
		},
	}
	uc := &AlipayNotifyUC{
		repo: repo,
		cfg: AlipayWebConfig{
			AppID: "9021000163631795",
			PID:   "2088721101056182",
		}.Normalize(),
	}

	err := uc.Execute(context.Background(), AlipayNotifyCmd{
		AppID:       "9021000163631795",
		SellerID:    "2088721101056182",
		OutTradeNo:  "1001",
		TradeNo:     "ALI-1",
		TradeStatus: "TRADE_SUCCESS",
		TotalAmount: "99.00",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestAlipayNotifyUC_RejectsInvalidAppID(t *testing.T) {
	repo := &notifyTestRepo{
		payment: domain.Payment{
			BizTradeNo: "1001",
			Status:     domain.PaymentStatusInit,
			Amt: domain.Amount{
				Currency: "CNY",
				Total:    9900,
			},
		},
	}
	uc := &AlipayNotifyUC{
		repo: repo,
		cfg: AlipayWebConfig{
			AppID: "9021000163631795",
			PID:   "2088721101056182",
		}.Normalize(),
	}

	err := uc.Execute(context.Background(), AlipayNotifyCmd{
		AppID:       "wrong-app",
		SellerID:    "2088721101056182",
		OutTradeNo:  "1001",
		TradeNo:     "ALI-1",
		TradeStatus: "TRADE_SUCCESS",
		TotalAmount: "99.00",
	})
	if !errors.Is(err, domain.ErrInvalidNotifyData) {
		t.Fatalf("expected ErrInvalidNotifyData, got %v", err)
	}
}

func TestAlipayNotifyUC_AmountMismatch(t *testing.T) {
	repo := &notifyTestRepo{
		payment: domain.Payment{
			BizTradeNo: "1001",
			Status:     domain.PaymentStatusInit,
			Amt: domain.Amount{
				Currency: "CNY",
				Total:    9900,
			},
		},
	}
	uc := &AlipayNotifyUC{
		repo: repo,
		cfg: AlipayWebConfig{
			AppID: "9021000163631795",
			PID:   "2088721101056182",
		}.Normalize(),
		callbackUC: &PayCallbackUC{},
	}

	err := uc.Execute(context.Background(), AlipayNotifyCmd{
		AppID:       "9021000163631795",
		SellerID:    "2088721101056182",
		OutTradeNo:  "1001",
		TradeNo:     "ALI-1",
		TradeStatus: "TRADE_SUCCESS",
		TotalAmount: "88.00",
	})
	if !errors.Is(err, domain.ErrPaymentAmountChanged) {
		t.Fatalf("expected ErrPaymentAmountChanged, got %v", err)
	}
}
