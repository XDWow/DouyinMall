package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

func ensurePaymentRecord(ctx context.Context, repo domain.PaymentRepository, pmt domain.Payment) error {
	if err := repo.AddPayment(ctx, pmt); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrPaymentAlreadyExists) {
		return err
	}

	existing, err := repo.GetPayment(ctx, pmt.BizTradeNo)
	if err != nil {
		return err
	}
	if existing.Status == domain.PaymentStatusSuccess {
		return domain.ErrPaymentAlreadyPaid
	}
	if existing.Amt.Total != pmt.Amt.Total || existing.Amt.Currency != pmt.Amt.Currency {
		return domain.ErrPaymentAmountChanged
	}
	return nil
}
