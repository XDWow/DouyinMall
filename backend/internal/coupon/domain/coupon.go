package domain

import (
	"time"
)

type CouponScope uint8

const (
	CouponScopeUnknown  CouponScope = iota
	CouponScopeAll                  // 全场券（不限制）
	CouponScopeMerchant             // 商家券
	CouponScopeCategory             // 品类券
	CouponScopeProduct              // 商品券
)

// 优惠券类型
type CouponType uint8

const (
	CouponTypeUnspecified CouponType = iota
	CouponTypeAmount                 // 满减券（满100减20）
	CouponTypePercent                // 折扣券（8折）
	CouponTypeFixed                  // 立减券（无门槛减10元）
)

// 优惠券状态
type CouponStatus uint8

const (
	UserCouponStatusUnspecified CouponStatus = iota
	UserCouponStatusUnused                   // 未使用
	UserCouponStatusLocked                   // 已锁定（预扣中）
	UserCouponStatusUsed                     // 已使用
	UserCouponStatusExpired                  // 已过期
)

// 优惠券模板（领域实体）
// 管理员创建的优惠规则定义
type CouponTemplate struct {
	ID   int64
	Name string
	Type CouponType

	Threshold     int64 // 使用门槛（分），0 表示无门槛
	DiscountValue int64 // 金额（分）或折扣（80=8折）
	MaxDiscount   int64 // 折扣券最大优惠（分），0 表示不限制

	// 使用范围（互斥）
	Scope       CouponScope
	MerchantIDs []int64
	CategoryIDs []int64
	ProductIDs  []int64

	// 有效期（两种模式互斥）
	ValidDays      int64 // 相对有效期（天）
	ValidStartTime int64 // 固定开始时间（Unix 秒）
	ValidEndTime   int64 // 固定结束时间（Unix 秒）

	// 发放限制
	TotalCount   int32 // 发行总量，-1 表示不限
	IssuedCount  int32 // 已发放数量（运行态）
	PerUserLimit int32 // 每人限领

	Enabled bool
}

// CanIssue 检查是否可以发放，这是优惠券层面的是否能发放
func (t *CouponTemplate) CanIssue() bool {
	if !t.Enabled {
		return false
	}
	if t.TotalCount > 0 && t.IssuedCount >= t.TotalCount {
		return false
	}
	return true
}

// 计算用户券的有效期
func (t *CouponTemplate) CalculateValidTime() (start, end int64) {
	now := time.Now().Unix()
	if t.ValidDays > 0 {
		// 领取后N天有效
		start = now
		end = now + t.ValidDays*86400
	} else {
		// 固定时间段
		start = t.ValidStartTime
		end = t.ValidEndTime
	}
	return
}

type Coupon struct {
	ID             int64
	UserID         int64
	TemplateID     int64
	Template       *CouponTemplate // 关联的模板
	Status         CouponStatus
	LockedOrderID  int64 // 锁定的订单
	UsedOrderID    int64 // 使用的订单
	ValidStartTime int64
	ValidEndTime   int64
	CreatedAt      int64
	UsedAt         int64
}

// DDD:Tell, Don't Ask，告诉我你要做什么，我来规约，我觉得哪些状态可以转移，你告诉我你要做什么

// Reserve 预扣优惠券（状态转移：Unused → Locked）
func (c *Coupon) Reserve(orderID int64) error {
	if c.Status != UserCouponStatusUnused {
		return ErrCouponNotAvailable
	}
	c.Status = UserCouponStatusLocked
	c.LockedOrderID = orderID
	return nil
}

// Commit 确认使用（状态转移：Locked → Used）
func (c *Coupon) Commit() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUsed
	c.UsedOrderID = c.LockedOrderID
	return nil
}

// Release 释放优惠券（状态转移：Locked → Unused）
func (c *Coupon) Release() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUnused
	c.LockedOrderID = 0
	return nil
}

// OrderItem 订单商品（用于计算优惠券适用金额）
type OrderItem struct {
	ProductID  int64
	MerchantID int64
	CategoryID int64
	Subtotal   int64 // 该商品的小计金额（分）
}

// 检查优惠券是否适用于某个订单（计算适用金额 + 检查门槛）
func (t *CouponTemplate) IsApplicableToOrder(items []OrderItem) (bool, string) {
	// 计算适用金额，按照scope汇总金额
	applicableAmount := t.CalculateApplicableAmount(items)

	if applicableAmount == 0 {
		return false, "订单中无适用商品"
	}
	if applicableAmount < t.Threshold {
		return false, "未达到使用门槛"
	}

	return true, ""
}

// 计算优惠券在本订单中的适用金额
func (t *CouponTemplate) CalculateApplicableAmount(items []OrderItem) int64 {
	switch t.Scope {
	case CouponScopeAll:
		// 全场券：所有商品原价总和
		var total int64
		for _, item := range items {
			total += item.Subtotal
		}
		return total

	case CouponScopeMerchant:
		// 商家券：筛选指定商家的商品
		return sumByIDs(items, t.MerchantIDs, func(item OrderItem) int64 { return item.MerchantID })

	case CouponScopeCategory:
		// 品类券：筛选指定品类的商品
		return sumByIDs(items, t.CategoryIDs, func(item OrderItem) int64 { return item.CategoryID })

	case CouponScopeProduct:
		// 商品券：筛选指定商品
		return sumByIDs(items, t.ProductIDs, func(item OrderItem) int64 { return item.ProductID })

	default:
		return 0
	}
}

// 自定义 getID 的通用求和方法
func sumByIDs(items []OrderItem, ids []int64, getID func(OrderItem) int64) int64 {
	if len(ids) == 0 {
		return 0
	}
	// 空间换时间，快速判断订单项是否在ids中
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

// 优惠券操作记录，用于幂等和审计
type CouponOperation struct {
	ID           int64
	OperationID  string // 业务幂等键
	UserCouponID int64
	OrderID      int64
	Type         string // reserve/commit/release/refund
	CreatedAt    int64
}
