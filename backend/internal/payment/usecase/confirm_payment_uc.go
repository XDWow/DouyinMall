package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type ConfirmPaymentUC struct {
	repo   domain.PaymentRepository
	syncUC *SyncWechatOrderUC
}

func NewConfirmPaymentUC(
	repo domain.PaymentRepository,
	syncUC *SyncWechatOrderUC,
) *ConfirmPaymentUC {
	return &ConfirmPaymentUC{
		repo:   repo,
		syncUC: syncUC,
	}
}

func (uc *ConfirmPaymentUC) Execute(ctx context.Context, bizTradeNo string) (domain.Payment, error) {
	pmt, err := uc.repo.GetPayment(ctx, bizTradeNo)
	if err != nil {
		return domain.Payment{}, err
	}

	switch pmt.Status {
	case domain.PaymentStatusSuccess:
		return pmt, nil
	case domain.PaymentStatusInit:
		if err = uc.syncUC.SyncWechatInfo(ctx, bizTradeNo); err != nil {
			return domain.Payment{}, err
		}
		return uc.repo.GetPayment(ctx, bizTradeNo)
	default:
		return pmt, nil
	}
}


