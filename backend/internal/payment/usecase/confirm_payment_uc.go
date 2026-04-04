package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type ConfirmPaymentUC struct {
	repo       domain.PaymentRepository
	syncUC     *SyncWechatOrderUC
	callbackUC *PayCallbackUC
}

func NewConfirmPaymentUC(
	repo domain.PaymentRepository,
	syncUC *SyncWechatOrderUC,
	callbackUC *PayCallbackUC,
) *ConfirmPaymentUC {
	return &ConfirmPaymentUC{
		repo:       repo,
		syncUC:     syncUC,
		callbackUC: callbackUC,
	}
}

func (uc *ConfirmPaymentUC) Execute(ctx context.Context, bizTradeNo string) (domain.Payment, error) {
	pmt, err := uc.repo.GetPayment(ctx, bizTradeNo)
	if err != nil {
		return domain.Payment{}, err
	}

	switch pmt.Status {
	case domain.PaymentStatusSuccess:
		if err = uc.callbackUC.UpdatePaymentByTxn(ctx, CallbackCmd{
			TradeState:    "SUCCESS",
			TransactionId: pmt.TxnID,
			OutTradeNo:    pmt.BizTradeNo,
		}); err != nil {
			return domain.Payment{}, err
		}
		return uc.repo.GetPayment(ctx, bizTradeNo)
	case domain.PaymentStatusInit:
		if err = uc.syncUC.SyncWechatInfo(ctx, bizTradeNo); err != nil {
			return domain.Payment{}, err
		}
		return uc.repo.GetPayment(ctx, bizTradeNo)
	default:
		return pmt, nil
	}
}


