package domain

import "context"

// WechatNativeService 寰俊Native鏀粯鏈嶅姟鎺ュ彛
// 鐢ㄤ簬瑙ｈ€seCase涓庡叿浣撶殑SDK瀹炵幇锛屾柟渚挎祴璇?
type WechatNativeService interface {
	// Prepay 棰勬敮浠橈紝杩斿洖浜岀淮鐮乁RL
	Prepay(ctx context.Context, req PrepayRequest) (codeURL string, err error)
	// QueryOrderByOutTradeNo 閫氳繃鍟嗘埛璁㈠崟鍙锋煡璇㈣鍗?
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*WechatOrder, error)
}

// PrepayRequest 棰勬敮浠樿姹?
type PrepayRequest struct {
	AppID       string
	MchID       string
	Description string
	OutTradeNo  string
	NotifyURL   string
	Amount      Amount
	TimeExpire  int64
}

// WechatOrder 寰俊璁㈠崟淇℃伅
type WechatOrder struct {
	OutTradeNo     string
	TransactionID  string
	TradeState     string
	TradeStateDesc string
	Amount         Amount
}


