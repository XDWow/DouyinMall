package wechat

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

// WechatNativeAdapter 微信Native支付服务的真实实现
type WechatNativeAdapter struct {
	svc *native.NativeApiService
}

func NewWechatNativeService(svc *native.NativeApiService) *WechatNativeAdapter {
	return &WechatNativeAdapter{svc: svc}
}

func (w *WechatNativeAdapter) Prepay(ctx context.Context, req domain.PrepayRequest) (string, error) {
	resp, _, err := w.svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(req.AppID),
		Mchid:       core.String(req.MchID),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(req.OutTradeNo),
		NotifyUrl:   core.String(req.NotifyURL),
		TimeExpire:  core.Time(time.Unix(req.TimeExpire, 0)),
		Amount: &native.Amount{
			Total:    core.Int64(req.Amount.Total),
			Currency: core.String(req.Amount.Currency),
		},
	})
	if err != nil {
		return "", err
	}
	return *resp.CodeUrl, nil
}

func (w *WechatNativeAdapter) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.WechatOrder, error) {
	resp, _, err := w.svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(outTradeNo),
	})
	if err != nil {
		return nil, err
	}
	return &domain.WechatOrder{
		OutTradeNo:     *resp.OutTradeNo,
		TransactionID:  *resp.TransactionId,
		TradeState:     *resp.TradeState,
		TradeStateDesc: *resp.TradeStateDesc,
		Amount: domain.Amount{
			Total:    *resp.Amount.Total,
			Currency: *resp.Amount.Currency,
		},
	}, nil
}
