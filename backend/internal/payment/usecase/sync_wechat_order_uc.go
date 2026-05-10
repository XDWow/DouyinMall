package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type PaymentOrderResult struct {
	TradeState    string
	TransactionId string
	OutTradeNo    string
}

type SyncPaymentOrderUC struct {
	svc           domain.NativePayService
	payCallbackUC *PayCallbackUC
	l             logger.LoggerV1
}

func NewSyncPaymentOrderUC(
	svc domain.NativePayService,
	payCallbackUC *PayCallbackUC,
	l logger.LoggerV1,
) *SyncPaymentOrderUC {
	return &SyncPaymentOrderUC{
		svc:           svc,
		payCallbackUC: payCallbackUC,
		l:             l,
	}
}

func (uc *SyncPaymentOrderUC) SyncOrderInfo(ctx context.Context, bizTradeNo string) error {
	resp, err := uc.svc.QueryOrderByOutTradeNo(ctx, bizTradeNo)
	if err != nil {
		uc.l.Error("查询支付渠道订单失败",
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	cmd := CallbackCmd{
		TradeState:    resp.TradeState,
		TransactionId: resp.TransactionID,
		OutTradeNo:    resp.OutTradeNo,
	}

	if err = uc.payCallbackUC.UpdatePaymentByTxn(ctx, cmd); err != nil {
		uc.l.Error("根据支付渠道订单更新本地支付状态失败",
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	return nil
}
