package domain

import "context"

// PagePayService 抽象了电脑网站支付能力。
// 当前返回的是前端可直接跳转的收银台 URL。
type PagePayService interface {
	BuildPagePayURL(ctx context.Context, req PagePayRequest) (string, error)
}

type PagePayRequest struct {
	OutTradeNo  string
	Subject     string
	TotalAmount string
	SellerID    string
	NotifyURL   string
	ReturnURL   string
}
