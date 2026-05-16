package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type paymentRecordTestRepo struct {
	addErr     error
	getPayment domain.Payment
	getErr     error
	addCalls   int
	getCalls   int
}

func (r *paymentRecordTestRepo) AddPayment(context.Context, domain.Payment) error {
	r.addCalls++
	return r.addErr
}

func (r *paymentRecordTestRepo) UpdatePayment(context.Context, domain.Payment) error {
	return nil
}

func (r *paymentRecordTestRepo) ApplyProviderResult(context.Context, domain.Payment) (domain.Payment, bool, error) {
	return domain.Payment{}, false, nil
}

func (r *paymentRecordTestRepo) GetPayment(context.Context, string) (domain.Payment, error) {
	r.getCalls++
	if r.getErr != nil {
		return domain.Payment{}, r.getErr
	}
	return r.getPayment, nil
}

func (r *paymentRecordTestRepo) FindExpiredPayment(context.Context, int, time.Time) ([]domain.Payment, error) {
	return nil, nil
}

func TestEnsurePaymentRecordPrefersInsertOnFastPath(t *testing.T) {
	repo := &paymentRecordTestRepo{}
	pmt := domain.Payment{
		BizTradeNo: "1001",
		Amt: domain.Amount{
			Currency: "CNY",
			Total:    9900,
		},
	}

	err := ensurePaymentRecord(context.Background(), repo, pmt)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.addCalls != 1 {
		t.Fatalf("add calls=%d, want 1", repo.addCalls)
	}
	if repo.getCalls != 0 {
		t.Fatalf("get calls=%d, want 0", repo.getCalls)
	}
}

func TestEnsurePaymentRecordLoadsExistingWhenDuplicateInsertOccurs(t *testing.T) {
	repo := &paymentRecordTestRepo{
		addErr: domain.ErrPaymentAlreadyExists,
		getPayment: domain.Payment{
			BizTradeNo: "1001",
			Status:     domain.PaymentStatusInit,
			Amt: domain.Amount{
				Currency: "CNY",
				Total:    9900,
			},
		},
	}
	pmt := domain.Payment{
		BizTradeNo: "1001",
		Amt: domain.Amount{
			Currency: "CNY",
			Total:    9900,
		},
	}

	err := ensurePaymentRecord(context.Background(), repo, pmt)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.addCalls != 1 {
		t.Fatalf("add calls=%d, want 1", repo.addCalls)
	}
	if repo.getCalls != 1 {
		t.Fatalf("get calls=%d, want 1", repo.getCalls)
	}
}

func TestEnsurePaymentRecordRejectsMismatchedExistingAmount(t *testing.T) {
	repo := &paymentRecordTestRepo{
		addErr: domain.ErrPaymentAlreadyExists,
		getPayment: domain.Payment{
			BizTradeNo: "1001",
			Status:     domain.PaymentStatusInit,
			Amt: domain.Amount{
				Currency: "CNY",
				Total:    8800,
			},
		},
	}
	pmt := domain.Payment{
		BizTradeNo: "1001",
		Amt: domain.Amount{
			Currency: "CNY",
			Total:    9900,
		},
	}

	err := ensurePaymentRecord(context.Background(), repo, pmt)
	if !errors.Is(err, domain.ErrPaymentAmountChanged) {
		t.Fatalf("expected ErrPaymentAmountChanged, got %v", err)
	}
}
