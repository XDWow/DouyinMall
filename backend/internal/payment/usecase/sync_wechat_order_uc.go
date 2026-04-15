package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type WechatOrderResult struct {
	TradeState    string
	TransactionId string
	OutTradeNo    string
}

type SyncWechatOrderUC struct {
	svc           domain.WechatNativeService
	payCallbackUC *PayCallbackUC
	l             logger.LoggerV1
}

func NewSyncWechatOrderUC(
	svc domain.WechatNativeService,
	payCallbackUC *PayCallbackUC,
	l logger.LoggerV1,
) *SyncWechatOrderUC {
	return &SyncWechatOrderUC{
		svc:           svc,
		payCallbackUC: payCallbackUC,
		l:             l,
	}
}

func (uc *SyncWechatOrderUC) SyncWechatInfo(ctx context.Context, bizTradeNo string) error {
	resp, err := uc.svc.QueryOrderByOutTradeNo(ctx, bizTradeNo)
	if err != nil {
		uc.l.Error("查询微信支付单失败",
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
		uc.l.Error("根据微信订单更新本地支付记录失败",
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	return nil
}


