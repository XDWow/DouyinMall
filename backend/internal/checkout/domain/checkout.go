package domain

// CheckoutItem 前端传入的商品项（只有ID和数量）
type CheckoutItem struct {
	ProductID int64
	Quantity  int64
}

// 从 Coupon 服务获取的可用优惠券
type CouponOption struct {
	CouponID         int64
	Name             string
	Description      string          // 如 "满100减20"
	DiscountAmount   int64           // 总优惠金额（分）= sum(PerLineDiscounts)
	PerLineDiscounts map[int64]int64 // key: ProductID，value: 该行商品优惠金额（分）
	Usable           bool            // 是否可用于本订单
	UnusableReason   string          // 不可用原因（未达门槛等）
}

// Address 收货地址
type Address struct {
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     string
	Street       string
	ZipCode      string
}

// ==================== 核心领域对象 ====================

// OrderLine 订单行（商品 + 价格快照 + 数量）
// 是 CheckoutItem + Product 合并后的视图
// 既是 PreviewOrder 的返回数据，也是 PlaceOrder 传给 Order 服务的数据
type OrderLine struct {
	ProductID     int64
	Name          string
	Picture       string
	Price         int64 // 下单时的价格快照（分），锁定此刻的价格
	Currency      string
	Quantity      int64
	Subtotal      int64 // Price × Quantity（分）
	Available     bool  // 当前是否可购买
	UnavailReason string
}

// PriceDetail 价格明细（核心值对象，所有计算基于此）
type PriceDetail struct {
	ProductAmount  int64 // 商品总价（分）= Σ(line.Subtotal)
	CouponDiscount int64 // 优惠券抵扣金额（分）
	TotalAmount    int64 // 应付金额（分）= ProductAmount - CouponDiscount
}

// CheckoutContext PlaceOrder 的完整上下文
// 把分散的入参聚合成一个领域对象，UseCase 围绕它编排
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

// ==================== 业务逻辑（Tell, Don't Ask）====================

// CalculatePrice 根据订单行和优惠券抵扣金额计算价格明细。
// couponDiscount 由 Coupon 服务计算后传入，这里只做汇总。
func CalculatePrice(lines []OrderLine, couponDiscount int64) PriceDetail {
	var productAmount int64
	for _, line := range lines {
		if line.Available {
			productAmount += line.Subtotal
		}
	}
	total := productAmount - couponDiscount
	if total < 0 {
		total = 0 // 优惠不能超过商品总价
	}
	return PriceDetail{
		ProductAmount:  productAmount,
		CouponDiscount: couponDiscount,
		TotalAmount:    total,
	}
}

// ValidatePriceChange 校验用户在结算页确认的价格与当前重算的价格是否一致。
// PlaceOrder 时调用，防止商品在 PreviewOrder 之后发生涨价被静默扣款。
// 允许降价（actual < expected），因为对用户有利。
func ValidatePriceChange(expected, actual int64) error {
	if actual > expected {
		return ErrPriceChanged
	}
	return nil
}
