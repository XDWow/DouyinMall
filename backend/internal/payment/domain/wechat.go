package domain

import "context"

// NativePayService abstracts the payment channel used by the payment service.
// It is implemented by mock wechat, real wechat, and alipay sandbox adapters.
type NativePayService interface {
	Prepay(ctx context.Context, req PrepayRequest) (codeURL string, err error)
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*PayOrder, error)
}

// Keep backward compatibility with existing code and tests.
type WechatNativeService = NativePayService

type PrepayRequest struct {
	AppID       string
	MchID       string
	Description string
	OutTradeNo  string
	NotifyURL   string
	Amount      Amount
	TimeExpire  int64
}

type PayOrder struct {
	OutTradeNo     string
	TransactionID  string
	TradeState     string
	TradeStateDesc string
	Amount         Amount
}

// Keep backward compatibility with existing adapters.
type WechatOrder = PayOrder
