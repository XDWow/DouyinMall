package domain

// CheckoutItem is the minimal item payload accepted from the client.
type CheckoutItem struct {
	ProductID int64
	SKUID     int64
	Quantity  int64
}

// CouponOption describes one coupon candidate returned to the checkout page.
type CouponOption struct {
	CouponID         int64
	Name             string
	Description      string
	DiscountAmount   int64
	PerLineDiscounts map[int64]int64 // key: ProductID
	Usable           bool
	UnusableReason   string
}

// Address stores the shipping address attached to an order.
type Address struct {
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     string
	Street       string
	ZipCode      string
}

// OrderLine is a checkout snapshot built from client input and product data.
type OrderLine struct {
	ProductID     int64
	SKUID         int64
	Name          string
	Picture       string
	Price         int64
	Currency      string
	Quantity      int64
	Subtotal      int64
	Available     bool
	UnavailReason string
}

// PriceDetail is the price calculation result used by the checkout flow.
type PriceDetail struct {
	ProductAmount  int64
	CouponDiscount int64
	TotalAmount    int64
}

// CheckoutContext groups the data needed during place-order orchestration.
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

// CalculatePrice aggregates line subtotals and coupon discount.
func CalculatePrice(lines []OrderLine, couponDiscount int64) PriceDetail {
	var productAmount int64
	for _, line := range lines {
		if line.Available {
			productAmount += line.Subtotal
		}
	}
	total := productAmount - couponDiscount
	if total < 0 {
		total = 0
	}
	return PriceDetail{
		ProductAmount:  productAmount,
		CouponDiscount: couponDiscount,
		TotalAmount:    total,
	}
}

// ValidatePriceChange prevents silent price increases between preview and place-order.
// Lower actual prices are allowed because they benefit the user.
func ValidatePriceChange(expected, actual int64) error {
	if actual > expected {
		return ErrPriceChanged
	}
	return nil
}
