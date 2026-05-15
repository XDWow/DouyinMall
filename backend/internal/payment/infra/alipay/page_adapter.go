package alipay

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	ali "github.com/smartwalle/alipay/v3"
)

type PageAdapter struct {
	client *ali.Client
}

func NewPagePayService(client *ali.Client) *PageAdapter {
	return &PageAdapter{client: client}
}

func (a *PageAdapter) BuildPagePayURL(_ context.Context, req domain.PagePayRequest) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("alipay client is nil")
	}

	pageURL, err := a.client.TradePagePay(ali.TradePagePay{
		Trade: ali.Trade{
			NotifyURL:   req.NotifyURL,
			ReturnURL:   req.ReturnURL,
			OutTradeNo:  req.OutTradeNo,
			Subject:     req.Subject,
			TotalAmount: req.TotalAmount,
			SellerId:    req.SellerID,
			ProductCode: "FAST_INSTANT_TRADE_PAY",
		},
		IntegrationType: "PCWEB",
	})
	if err != nil {
		return "", fmt.Errorf("build alipay page pay url failed: %w", err)
	}
	if pageURL == nil {
		return "", fmt.Errorf("build alipay page pay url failed: empty url")
	}
	return pageURL.String(), nil
}

var _ domain.PagePayService = (*PageAdapter)(nil)
