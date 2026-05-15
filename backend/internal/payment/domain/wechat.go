package domain

import "context"

// NativePayService 抽象了支付服务对接的原生支付通道。
// 当前既可以接 mock wechat，也可以接真实微信或支付宝沙盒。
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
