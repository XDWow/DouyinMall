package domain

import (
	"context"
	"time"
)

type PaymentRepository interface {
	AddPayment(ctx context.Context, pmt Payment) error
	UpdatePayment(ctx context.Context, pmt Payment) error
	ApplyProviderResult(ctx context.Context, pmt Payment) (Payment, bool, error)
	GetPayment(ctx context.Context, bizTradeNo string) (Payment, error)
	FindExpiredPayment(ctx context.Context, limit int, t time.Time) ([]Payment, error)
}

type PaymentOutboxRepository interface {
	Add(ctx context.Context, eventType string, payload any) (int64, error)
	ListPending(ctx context.Context, offset, limit int) ([]PaymentOutboxEvent, error)
	MarkSent(ctx context.Context, id int64) error
	BatchMarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64) error
	IncreaseRetry(ctx context.Context, id int64) (int, error)
}

type TxManager interface {
	Tx(ctx context.Context, fn func(ctx context.Context) error) error
}
