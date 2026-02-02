package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
)

// MockWechatNativeService 实现 domain.WechatNativeService 接口
// 它通过 HTTP 调用 Mock 微信服务器
type MockWechatNativeService struct {
	baseURL string
	client  *http.Client
}

func NewMockWechatNativeService(baseURL string) *MockWechatNativeService {
	return &MockWechatNativeService{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (m *MockWechatNativeService) Prepay(ctx context.Context, req domain.PrepayRequest) (string, error) {
	// 构建请求体
	body := map[string]interface{}{
		"appid":        req.AppID,
		"mchid":        req.MchID,
		"description":  req.Description,
		"out_trade_no": req.OutTradeNo,
		"notify_url":   req.NotifyURL,
		"amount": map[string]interface{}{
			"total":    req.Amount.Total,
			"currency": req.Amount.Currency,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request body failed: %w", err)
	}

	// 调用 Mock 服务器
	url := m.baseURL + "/v3/pay/transactions/native"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mock server returned status %d", resp.StatusCode)
	}

	var result struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	return result.CodeURL, nil
}

func (m *MockWechatNativeService) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*domain.WechatOrder, error) {
	url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=mock_mchid", m.baseURL, outTradeNo)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mock server returned status %d", resp.StatusCode)
	}

	var result struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		Amount        struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return &domain.WechatOrder{
		OutTradeNo:    result.OutTradeNo,
		TransactionID: result.TransactionID,
		TradeState:    result.TradeState,
		Amount: domain.Amount{
			Total:    result.Amount.Total,
			Currency: result.Amount.Currency,
		},
	}, nil
}
