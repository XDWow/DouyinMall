package alipay

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	ali "github.com/smartwalle/alipay/v3"
)

type NativeAdapter struct {
	client *ali.Client
}

func NewNativeService(client *ali.Client) *NativeAdapter {
	return &NativeAdapter{client: client}
}

func (a *NativeAdapter) Prepay(ctx context.Context, req domain.PrepayRequest) (string, error) {
	resp, err := a.client.TradePreCreate(ctx, ali.TradePreCreate{
		Trade: ali.Trade{
			NotifyURL:   req.NotifyURL,
			OutTradeNo:  req.OutTradeNo,
			Subject:     nonEmptySubject(req.Description),
			TotalAmount: centsToYuan(req.Amount.Total),
		},
	})
	if err != nil {
		return "", fmt.Errorf("alipay precreate failed: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("alipay precreate failed: empty response")
	}
	if resp.IsFailure() {
		return "", fmt.Errorf("alipay precreate failed: code=%s msg=%s sub_msg=%s", resp.Code, resp.Msg, resp.SubMsg)
	}
	if resp.QRCode == "" {
		return "", fmt.Errorf("alipay precreate failed: empty qr code")
	}
	return resp.QRCode, nil
}

func (a *NativeAdapter) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.PayOrder, error) {
	resp, err := a.client.TradeQuery(ctx, ali.TradeQuery{OutTradeNo: outTradeNo})
	if err != nil {
		return nil, fmt.Errorf("alipay trade query failed: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("alipay trade query failed: empty response")
	}
	if resp.IsFailure() {
		return nil, fmt.Errorf("alipay trade query failed: code=%s msg=%s sub_msg=%s", resp.Code, resp.Msg, resp.SubMsg)
	}

	return &domain.PayOrder{
		OutTradeNo:     resp.OutTradeNo,
		TransactionID:  resp.TradeNo,
		TradeState:     string(resp.TradeStatus),
		TradeStateDesc: resp.Msg,
		Amount: domain.Amount{
			Total:    yuanToCents(resp.TotalAmount),
			Currency: defaultCurrency(resp.TransCurrency),
		},
	}, nil
}

func defaultCurrency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return "CNY"
	}
	return currency
}

func nonEmptySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "DouyinMall Order"
	}
	return subject
}
