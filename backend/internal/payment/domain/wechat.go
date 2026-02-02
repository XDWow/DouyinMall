package domain

import "context"

// WechatNativeService 微信Native支付服务接口
// 用于解耦UseCase与具体的SDK实现，方便测试
type WechatNativeService interface {
	// Prepay 预支付，返回二维码URL
	Prepay(ctx context.Context, req PrepayRequest) (codeURL string, err error)
	// QueryOrderByOutTradeNo 通过商户订单号查询订单
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*WechatOrder, error)
}

// PrepayRequest 预支付请求
type PrepayRequest struct {
	AppID       string
	MchID       string
	Description string
	OutTradeNo  string
	NotifyURL   string
	Amount      Amount
	TimeExpire  int64
}

// WechatOrder 微信订单信息
type WechatOrder struct {
	OutTradeNo     string
	TransactionID  string
	TradeState     string
	TradeStateDesc string
	Amount         Amount
}
