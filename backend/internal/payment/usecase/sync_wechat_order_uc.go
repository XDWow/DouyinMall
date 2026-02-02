package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

type WechatOrderResult struct {
	TradeState    string // 交易状态
	TransactionId string // 微信支付订单号
	OutTradeNo    string // 商户订单号
}

// 主动查询微信订单状态，并更新本地支付记录
type SyncWechatOrderUC struct {
	svc           *native.NativeApiService
	mchID         string // 这个订单的钱归属方才能查订单状态
	payCallbackUC *PayCallbackUC
	l             logger.LoggerV1
}

func NewSyncWechatOrderUC(
	svc *native.NativeApiService,
	payCallbackUC *PayCallbackUC,
	l logger.LoggerV1,
	mchID string,
) *SyncWechatOrderUC {
	return &SyncWechatOrderUC{
		svc:           svc,
		payCallbackUC: payCallbackUC,
		l:             l,
		mchID:         mchID,
	}
}

func (uc *SyncWechatOrderUC) SyncWechatInfo(ctx context.Context, bizTradeNo string) error {
	resp, _, err := uc.svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(bizTradeNo),
		Mchid:      core.String(uc.mchID),
	})
	if err != nil {
		uc.l.Error("查询微信订单失败",
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	cmd := CallbackCmd{
		TradeState:    *resp.TradeState,
		TransactionId: *resp.TransactionId,
		OutTradeNo:    *resp.OutTradeNo,
	}

	err = uc.payCallbackUC.UpdatePaymentByTxn(ctx, cmd)
	if err != nil {
		uc.l.Error("更新支付状态失败",
			logger.String("biz_trade_no", bizTradeNo),
			logger.Error(err))
		return err
	}

	return nil
}
