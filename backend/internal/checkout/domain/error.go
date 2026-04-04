package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput 璇锋眰鍙傛暟涓嶅悎娉曪紙绌哄晢鍝佸垪琛ㄧ瓑锛?
	ErrInvalidInput = errors.New("invalid input")

	// ErrPriceChanged 浠?PreviewOrder 鍒?PlaceOrder 鏈熼棿鍟嗗搧浠锋牸涓婃定
	// 闇€瑕佺敤鎴烽噸鏂拌繘鍏ョ粨绠楅〉纭鏂颁环鏍?
	ErrPriceChanged = errors.New("price has changed, please re-confirm")

	// ErrInsufficientStock 闄嶇骇鍏滃簳锛歳esp 鏃犳槑缁嗘椂鐢?
	ErrInsufficientStock = errors.New("insufficient stock")

	// ErrOrderCreateFailed 鍒涘缓璁㈠崟澶辫触锛圤rder 鏈嶅姟杩斿洖锛孲aga 琛ュ伩瑙﹀彂锛?
	ErrOrderCreateFailed = errors.New("failed to create order")

	// ErrPaymentCreateFailed 鍒涘缓鏀粯鍗曞け璐ワ紙Payment 鏈嶅姟杩斿洖锛孲aga 琛ュ伩瑙﹀彂锛?
	ErrPaymentCreateFailed = errors.New("failed to create payment")
	ErrOrderNotPayable     = errors.New("order is not payable")
	ErrOrderExpired        = errors.New("order has expired")
	ErrOrderForbidden      = errors.New("order does not belong to current user")
)

// UnavailableItem 鍗曚釜澶辨晥鍟嗗搧鐨勮鎯?
type UnavailableItem struct {
	ProductID int64
	Name      string
	Reason    string // "鍟嗗搧宸蹭笅鏋? / "搴撳瓨涓嶈冻"
}

// UnavailableItemsError 闄勫甫鍏蜂綋澶辨晥鍟嗗搧鍒楄〃鐨勭粨鏋勫寲閿欒銆?
// 鍓嶇鏀跺埌鍚庡睍绀哄脊妗嗭紝鐢ㄦ埛纭绉婚櫎澶辨晥鍟嗗搧鍚庨噸鏂版彁浜ゃ€?
type UnavailableItemsError struct {
	Items []UnavailableItem
}

func (e *UnavailableItemsError) Error() string {
	return fmt.Sprintf("%d 浠跺晢鍝佸凡澶辨晥锛岃纭鍚庨噸鏂版彁浜?, len(e.Items))
}

// ==================== 搴撳瓨涓嶈冻 ====================

// InsufficientStockItem 鍗曚釜搴撳瓨涓嶈冻鍟嗗搧鐨勮鎯?
type InsufficientStockItem struct {
	ProductID int64
	Name      string
	Requested int64 // 鐢ㄦ埛涓嬪崟鏁伴噺
	Available int64 // 鍙敤搴撳瓨 = 瀹為檯搴撳瓨 - Redis 棰勬墸搴撳瓨
}

// InsufficientStockError 棰勬墸搴撳瓨澶辫触锛宨nventory 鏈嶅姟鐩存帴杩斿洖涓嶈冻鏄庣粏銆?
// 鍓嶇鍙睍绀?"XX 搴撳瓨浠呭墿 N 浠? 璁╃敤鎴疯皟鏁存暟閲忔垨绉婚櫎銆?
type InsufficientStockError struct {
	Items []InsufficientStockItem
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("%d 浠跺晢鍝佸簱瀛樹笉瓒?, len(e.Items))
}

// ==================== 浼樻儬鍒镐笉鍙敤 ====================

// CouponFailureItem 鍗曞紶鍒搁鎵ｅけ璐ョ殑璇︽儏
type CouponFailureItem struct {
	CouponID int64
	Reason   string // 鏉ヨ嚜 coupon 鏈嶅姟杩斿洖鐨?reason
}

// CouponUnavailableError 鎵归噺棰勬墸浼樻儬鍒稿け璐ョ殑缁撴瀯鍖栭敊璇€?
// coupon 鏈嶅姟浜嬪姟鍐呮壒閲忛鎵ｏ紝鍏ㄩ儴澶辫触鎴栭儴鍒嗗け璐ユ椂杩斿洖澶辫触鏄庣粏銆?
type CouponUnavailableError struct {
	Failures []CouponFailureItem
}

func (e *CouponUnavailableError) Error() string {
	return fmt.Sprintf("%d 寮犱紭鎯犲埜涓嶅彲鐢?, len(e.Failures))
}


