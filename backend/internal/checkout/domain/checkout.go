package domain

// CheckoutItem 鍓嶇浼犲叆鐨勫晢鍝侀」锛堝彧鏈塈D鍜屾暟閲忥級
type CheckoutItem struct {
	ProductID int64
	Quantity  int64
}

// 浠?Coupon 鏈嶅姟鑾峰彇鐨勫彲鐢ㄤ紭鎯犲埜
type CouponOption struct {
	CouponID         int64
	Name             string
	Description      string          // 濡?"婊?00鍑?0"
	DiscountAmount   int64           // 鎬讳紭鎯犻噾棰濓紙鍒嗭級= sum(PerLineDiscounts)
	PerLineDiscounts map[int64]int64 // key: ProductID锛寁alue: 璇ヨ鍟嗗搧浼樻儬閲戦锛堝垎锛?
	Usable           bool            // 鏄惁鍙敤浜庢湰璁㈠崟
	UnusableReason   string          // 涓嶅彲鐢ㄥ師鍥狅紙鏈揪闂ㄦ绛夛級
}

// Address 鏀惰揣鍦板潃
type Address struct {
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     string
	Street       string
	ZipCode      string
}

// ==================== 鏍稿績棰嗗煙瀵硅薄 ====================

// OrderLine 璁㈠崟琛岋紙鍟嗗搧 + 浠锋牸蹇収 + 鏁伴噺锛?
// 鏄?CheckoutItem + Product 鍚堝苟鍚庣殑瑙嗗浘
// 鏃㈡槸 PreviewOrder 鐨勮繑鍥炴暟鎹紝涔熸槸 PlaceOrder 浼犵粰 Order 鏈嶅姟鐨勬暟鎹?
type OrderLine struct {
	ProductID     int64
	Name          string
	Picture       string
	Price         int64 // 涓嬪崟鏃剁殑浠锋牸蹇収锛堝垎锛夛紝閿佸畾姝ゅ埢鐨勪环鏍?
	Currency      string
	Quantity      int64
	Subtotal      int64 // Price 脳 Quantity锛堝垎锛?
	Available     bool  // 褰撳墠鏄惁鍙喘涔?
	UnavailReason string
}

// PriceDetail 浠锋牸鏄庣粏锛堟牳蹇冨€煎璞★紝鎵€鏈夎绠楀熀浜庢锛?
type PriceDetail struct {
	ProductAmount  int64 // 鍟嗗搧鎬讳环锛堝垎锛? 危(line.Subtotal)
	CouponDiscount int64 // 浼樻儬鍒告姷鎵ｉ噾棰濓紙鍒嗭級
	TotalAmount    int64 // 搴斾粯閲戦锛堝垎锛? ProductAmount - CouponDiscount
}

// CheckoutContext PlaceOrder 鐨勫畬鏁翠笂涓嬫枃
// 鎶婂垎鏁ｇ殑鍏ュ弬鑱氬悎鎴愪竴涓鍩熷璞★紝UseCase 鍥寸粫瀹冪紪鎺?
type CheckoutContext struct {
	OrderID         int64
	UserID          int64
	Lines           []OrderLine
	SelectedCoupons []int64
	Price           PriceDetail
	Address         Address
	PaymentMethod   string
	Currency        string
	Remark          string
}

// ==================== 涓氬姟閫昏緫锛圱ell, Don't Ask锛?===================

// CalculatePrice 鏍规嵁璁㈠崟琛屽拰浼樻儬鍒告姷鎵ｉ噾棰濊绠椾环鏍兼槑缁嗐€?
// couponDiscount 鐢?Coupon 鏈嶅姟璁＄畻鍚庝紶鍏ワ紝杩欓噷鍙仛姹囨€汇€?
func CalculatePrice(lines []OrderLine, couponDiscount int64) PriceDetail {
	var productAmount int64
	for _, line := range lines {
		if line.Available {
			productAmount += line.Subtotal
		}
	}
	total := productAmount - couponDiscount
	if total < 0 {
		total = 0 // 浼樻儬涓嶈兘瓒呰繃鍟嗗搧鎬讳环
	}
	return PriceDetail{
		ProductAmount:  productAmount,
		CouponDiscount: couponDiscount,
		TotalAmount:    total,
	}
}

// ValidatePriceChange 鏍￠獙鐢ㄦ埛鍦ㄧ粨绠楅〉纭鐨勪环鏍间笌褰撳墠閲嶇畻鐨勪环鏍兼槸鍚︿竴鑷淬€?
// PlaceOrder 鏃惰皟鐢紝闃叉鍟嗗搧鍦?PreviewOrder 涔嬪悗鍙戠敓娑ㄤ环琚潤榛樻墸娆俱€?
// 鍏佽闄嶄环锛坅ctual < expected锛夛紝鍥犱负瀵圭敤鎴锋湁鍒┿€?
func ValidatePriceChange(expected, actual int64) error {
	if actual > expected {
		return ErrPriceChanged
	}
	return nil
}


