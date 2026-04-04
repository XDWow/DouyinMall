package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

type MockWechatNativeAdapter struct {
	baseURL string
	mchID   string
	client  *http.Client
}

func NewMockWechatNativeService(baseURL, mchID string) *MockWechatNativeAdapter {
	return &MockWechatNativeAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		mchID:   mchID,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (m *MockWechatNativeAdapter) Prepay(ctx context.Context, req domain.PrepayRequest) (string, error) {
	body := map[string]any{
		"appid":        req.AppID,
		"mchid":        req.MchID,
		"description":  req.Description,
		"out_trade_no": req.OutTradeNo,
		"notify_url":   req.NotifyURL,
		"amount": map[string]any{
			"total":    req.Amount.Total,
			"currency": req.Amount.Currency,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal mock prepay request failed: %w", err)
	}

	reqURL := m.baseURL + "/v3/pay/transactions/native"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create mock prepay request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call mock prepay failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mock prepay returned status %d", resp.StatusCode)
	}

	var result struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode mock prepay response failed: %w", err)
	}

	return result.CodeURL, nil
}

func (m *MockWechatNativeAdapter) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.WechatOrder, error) {
	reqURL := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=%s",
		m.baseURL,
		url.PathEscape(outTradeNo),
		url.QueryEscape(m.mchID),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create mock query request failed: %w", err)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call mock query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mock query returned status %d", resp.StatusCode)
	}

	var result struct {
		OutTradeNo     string `json:"out_trade_no"`
		TransactionID  string `json:"transaction_id"`
		TradeState     string `json:"trade_state"`
		TradeStateDesc string `json:"trade_state_desc"`
		Amount         struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode mock query response failed: %w", err)
	}

	return &domain.WechatOrder{
		OutTradeNo:     result.OutTradeNo,
		TransactionID:  result.TransactionID,
		TradeState:     result.TradeState,
		TradeStateDesc: result.TradeStateDesc,
		Amount: domain.Amount{
			Total:    result.Amount.Total,
			Currency: result.Amount.Currency,
		},
	}, nil
}


