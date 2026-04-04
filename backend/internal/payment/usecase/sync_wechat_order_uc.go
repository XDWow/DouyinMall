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
		uc.l.Error("鏌ヨ寰俊鏀粯鍗曞け璐?,
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
		uc.l.Error("鏍规嵁寰俊鏀粯鍗曟洿鏂版湰鍦版敮浠樿褰曞け璐?,
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	return nil
}


