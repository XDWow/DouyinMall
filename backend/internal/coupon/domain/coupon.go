package domain

import "time"

type CouponScope uint8

const (
	CouponScopeUnknown CouponScope = iota
	CouponScopeAll
	CouponScopeMerchant
	CouponScopeCategory
	CouponScopeProduct
)

type CouponType uint8

const (
	CouponTypeUnspecified CouponType = iota
	CouponTypeAmount
	CouponTypePercent
	CouponTypeFixed
)

type CouponStatus uint8

func (c CouponStatus) AsUint8() uint8 {
	return uint8(c)
}

const (
	UserCouponStatusUnspecified CouponStatus = iota
	UserCouponStatusUnused
	UserCouponStatusLocked
	UserCouponStatusUsed
	UserCouponStatusExpired
)

type CouponTemplate struct {
	ID   int64
	Name string
	Type CouponType

	Threshold     int64
	DiscountValue int64
	MaxDiscount   int64

	Scope       CouponScope
	MerchantIDs []int64
	CategoryIDs []int64
	ProductIDs  []int64

	ValidDays      int64
	ValidStartTime int64
	ValidEndTime   int64

	TotalCount   int32
	IssuedCount  int32
	PerUserLimit int32

	Enabled bool
}

func (t *CouponTemplate) CanIssue() bool {
	if !t.Enabled {
		return false
	}
	if t.TotalCount > 0 && t.IssuedCount >= t.TotalCount {
		return false
	}
	return true
}

func (t *CouponTemplate) CalculateValidTime() (start, end int64) {
	now := time.Now().Unix()
	if t.ValidDays > 0 {
		return now, now + t.ValidDays*86400
	}
	return t.ValidStartTime, t.ValidEndTime
}

type Coupon struct {
	ID             int64
	UserID         int64
	TemplateID     int64
	Template       *CouponTemplate
	Status         CouponStatus
	OrderID        int64
	ValidStartTime time.Time
	ValidEndTime   time.Time
	CreatedAt      time.Time
	UsedAt         time.Time
}

func (c *Coupon) Reserve(orderID int64) error {
	if c.Status != UserCouponStatusUnused {
		return ErrCouponNotAvailable
	}
	c.Status = UserCouponStatusLocked
	c.OrderID = orderID
	return nil
}

func (c *Coupon) Commit() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUsed
	return nil
}

func (c *Coupon) Release() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUnused
	c.OrderID = 0
	return nil
}

type OrderItem struct {
	ProductID  int64
	MerchantID int64
	CategoryID int64
	Subtotal   int64
}

func (t *CouponTemplate) IsApplicableToOrder(items []OrderItem) (bool, string) {
	applicableAmount := t.CalculateApplicableAmount(items)
	if applicableAmount == 0 {
		return false, ErrOrderNotApplicable.Error()
	}
	if applicableAmount < t.Threshold {
		return false, ErrThresholdNotMet.Error()
	}
	return true, ""
}

func (t *CouponTemplate) CalculateApplicableAmount(items []OrderItem) int64 {
	switch t.Scope {
	case CouponScopeAll:
		var total int64
		for _, item := range items {
			total += item.Subtotal
		}
		return total
	case CouponScopeMerchant:
		return sumByIDs(items, t.MerchantIDs, func(item OrderItem) int64 { return item.MerchantID })
	case CouponScopeCategory:
		return sumByIDs(items, t.CategoryIDs, func(item OrderItem) int64 { return item.CategoryID })
	case CouponScopeProduct:
		return sumByIDs(items, t.ProductIDs, func(item OrderItem) int64 { return item.ProductID })
	default:
		return 0
	}
}

func sumByIDs(items []OrderItem, ids []int64, getID func(OrderItem) int64) int64 {
	if len(ids) == 0 {
		return 0
	}

	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	var total int64
	for _, item := range items {
		if _, ok := idSet[getID(item)]; ok {
			total += item.Subtotal
		}
	}
	return total
}

type CouponOperation struct {
	ID           int64
	OperationID  string
	UserCouponID int64
	OrderID      int64
	Type         string
	CreatedAt    int64
}
