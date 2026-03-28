package domain

import (
	"context"
	"time"
)

type PaymentRepository interface {
	AddPayment(ctx context.Context, pmt Payment) error
	UpdatePayment(ctx context.Context, pmt Payment) error
	GetPayment(ctx context.Context, bizTradeNo string) (Payment, error)
	FindExpiredPayment(ctx context.Context, limit int, t time.Time) ([]Payment, error)
}
