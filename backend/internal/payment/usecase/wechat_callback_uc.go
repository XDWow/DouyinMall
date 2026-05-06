package usecase

import (
	"context"
	"errors"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type PayCallbackUC struct {
	repo       domain.PaymentRepository
	outboxRepo domain.PaymentOutboxRepository
	tx         domain.TxManager
	l          logger.LoggerV1

	nativeCBTypeToStatus map[string]domain.PaymentStatus
}

func NewPayCallbackUC(
	repo domain.PaymentRepository,
	outboxRepo domain.PaymentOutboxRepository,
	tx domain.TxManager,
	l logger.LoggerV1,
) *PayCallbackUC {
	return &PayCallbackUC{
		repo:       repo,
		outboxRepo: outboxRepo,
		tx:         tx,
		l:          l,
		nativeCBTypeToStatus: map[string]domain.PaymentStatus{
			"SUCCESS":        domain.PaymentStatusSuccess,
			"PAYERROR":       domain.PaymentStatusFailed,
			"NOTPAY":         domain.PaymentStatusInit,
			"USERPAYING":     domain.PaymentStatusInit,
			"CLOSED":         domain.PaymentStatusFailed,
			"REVOKED":        domain.PaymentStatusFailed,
			"REFUND":         domain.PaymentStatusRefund,
			"WAIT_BUYER_PAY": domain.PaymentStatusInit,
			"TRADE_SUCCESS":  domain.PaymentStatusSuccess,
			"TRADE_FINISHED": domain.PaymentStatusSuccess,
			"TRADE_CLOSED":   domain.PaymentStatusFailed,
		},
	}
}

func (uc *PayCallbackUC) Execute(ctx context.Context, cmd CallbackCmd) error {
	return uc.UpdatePaymentByTxn(ctx, cmd)
}

func (uc *PayCallbackUC) UpdatePaymentByTxn(ctx context.Context, cmd CallbackCmd) error {
	status, ok := uc.nativeCBTypeToStatus[cmd.TradeState]
	if !ok {
		return errors.New("unknown trade state")
	}

	orderID, err := strconv.ParseInt(cmd.OutTradeNo, 10, 64)
	if err != nil {
		return err
	}

	err = uc.tx.Tx(ctx, func(ctx context.Context) error {
		// Persist payment state and outbox atomically, so order advancement is recoverable.
		pmt, changed, applyErr := uc.repo.ApplyProviderResult(ctx, domain.Payment{
			BizTradeNo: cmd.OutTradeNo,
			Status:     status,
			TxnID:      cmd.TransactionId,
		})
		if applyErr != nil {
			return applyErr
		}
		if !changed {
			uc.l.Info("payment provider result ignored by monotonic state machine",
				logger.String("bizTradeNo", cmd.OutTradeNo),
				logger.Int("incomingStatus", int(status)))
			return nil
		}

		_, addErr := uc.outboxRepo.Add(ctx, domain.EventTypePaymentStatusChanged, domain.PaymentStatusUpdateEvent{
			BizTradeNo: pmt.BizTradeNo,
			OrderID:    orderID,
			Status:     pmt.Status,
			TxnID:      pmt.TxnID,
		})
		return addErr
	})
	if err != nil {
		return err
	}
	return nil
}

type CallbackCmd struct {
	TradeState    string
	TransactionId string
	OutTradeNo    string
}
