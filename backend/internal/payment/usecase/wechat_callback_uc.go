package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"

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
	cmd.TradeState = strings.TrimSpace(cmd.TradeState)
	cmd.TransactionId = strings.TrimSpace(cmd.TransactionId)
	cmd.OutTradeNo = strings.TrimSpace(cmd.OutTradeNo)

	status, ok := uc.nativeCBTypeToStatus[cmd.TradeState]
	if !ok {
		return errors.New("unknown trade state")
	}
	if status == domain.PaymentStatusSuccess && cmd.TransactionId == "" {
		return domain.ErrPaymentTxnIDRequired
	}

	orderID, err := strconv.ParseInt(cmd.OutTradeNo, 10, 64)
	if err != nil {
		return err
	}

	err = uc.tx.Tx(ctx, func(ctx context.Context) error {
		// 支付状态和 outbox 事件放在同一个事务里，确保订单状态推进可恢复。
		pmt, changed, applyErr := uc.repo.ApplyProviderResult(ctx, domain.Payment{
			BizTradeNo: cmd.OutTradeNo,
			Status:     status,
			TxnID:      cmd.TransactionId,
		})
		if applyErr != nil {
			return applyErr
		}
		if !changed {
			uc.l.Info("支付回调被状态机忽略",
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
