package usecase

import (
	"context"
	"errors"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

type PayCallbackUC struct {
	repo     domain.PaymentRepository
	orderCli orderservice.Client
	l        logger.LoggerV1

	nativeCBTypeToStatus map[string]domain.PaymentStatus
}

func NewPayCallbackUC(repo domain.PaymentRepository, orderCli orderservice.Client) *PayCallbackUC {
	return &PayCallbackUC{
		repo:     repo,
		orderCli: orderCli,
		nativeCBTypeToStatus: map[string]domain.PaymentStatus{
			"SUCCESS":    domain.PaymentStatusSuccess,
			"PAYERROR":   domain.PaymentStatusFailed,
			"NOTPAY":     domain.PaymentStatusInit,
			"USERPAYING": domain.PaymentStatusInit,
			"CLOSED":     domain.PaymentStatusFailed,
			"REVOKED":    domain.PaymentStatusFailed,
			"REFUND":     domain.PaymentStatusRefund,
		},
	}
}

func (uc *PayCallbackUC) Execute(ctx context.Context, cmd CallbackCmd) error {
	return uc.UpdatePaymentByTxn(ctx, cmd)
}

func (uc *PayCallbackUC) UpdatePaymentByTxn(ctx context.Context, cmd CallbackCmd) error {
	status, ok := uc.nativeCBTypeToStatus[cmd.TradeState]
	if !ok {
		return errors.New("unknown wechat trade state")
	}

	if err := uc.repo.UpdatePayment(ctx, domain.Payment{
		BizTradeNo: cmd.OutTradeNo,
		Status:     status,
		TxnID:      cmd.TransactionId,
	}); err != nil {
		return err
	}

	if status != domain.PaymentStatusSuccess {
		return nil
	}

	orderID, err := strconv.ParseInt(cmd.OutTradeNo, 10, 64)
	if err != nil {
		return err
	}

	_, err = uc.orderCli.ChangeOrderStatus(ctx, &orderv1.ChangeOrderStatusReq{
		OrderId: orderID,
		Action:  orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_PAY,
	})
	if err != nil {
		uc.l.Error("change order status to paid failed", logger.Error(err), logger.Int64("orderID", orderID))
		return err
	}
	return nil
}

type CallbackCmd struct {
	TradeState    string
	TransactionId string
	OutTradeNo    string
}
