package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput 请求参数不合法（空商品列表等）
	ErrInvalidInput = errors.New("invalid input")

	// ErrPriceChanged 从 PreviewOrder 到 PlaceOrder 期间商品价格上涨
	// 需要用户重新进入结算页确认新价格
	ErrPriceChanged = errors.New("price has changed, please re-confirm")

	// ErrInsufficientStock 降级兜底：resp 无明细时用
	ErrInsufficientStock = errors.New("insufficient stock")

	// ErrOrderCreateFailed 创建订单失败（Order 服务返回，Saga 补偿触发）
	ErrOrderCreateFailed = errors.New("failed to create order")

	// ErrPaymentCreateFailed 创建支付单失败（Payment 服务返回，Saga 补偿触发）
	ErrPaymentCreateFailed = errors.New("failed to create payment")
	ErrOrderNotPayable     = errors.New("order is not payable")
	ErrOrderExpired        = errors.New("order has expired")
	ErrOrderForbidden      = errors.New("order does not belong to current user")
)

// UnavailableItem 单个失效商品的详情
type UnavailableItem struct {
	ProductID int64
	Name      string
	Reason    string // "商品已下架" / "库存不足"
}

// UnavailableItemsError 附带具体失效商品列表的结构化错误。
// 前端收到后展示弹框，用户确认移除失效商品后重新提交。
type UnavailableItemsError struct {
	Items []UnavailableItem
}

func (e *UnavailableItemsError) Error() string {
	return fmt.Sprintf("%d 件商品已失效，请确认后重新提交", len(e.Items))
}

// ==================== 库存不足 ====================

// InsufficientStockItem 单个库存不足商品的详情
type InsufficientStockItem struct {
	ProductID int64
	Name      string
	Requested int64 // 用户下单数量
	Available int64 // 可用库存 = 实际库存 - Redis 预扣库存
}

// InsufficientStockError 预扣库存失败，inventory 服务直接返回不足明细。
// 前端可展示 "XX 库存仅剩 N 件" 让用户调整数量或移除。
type InsufficientStockError struct {
	Items []InsufficientStockItem
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("%d 件商品库存不足", len(e.Items))
}

// ==================== 优惠券不可用 ====================

// CouponFailureItem 单张券预扣失败的详情
type CouponFailureItem struct {
	CouponID int64
	Reason   string // 来自 coupon 服务返回的 reason
}

// CouponUnavailableError 批量预扣优惠券失败的结构化错误。
// coupon 服务事务内批量预扣，全部失败或部分失败时返回失败明细。
type CouponUnavailableError struct {
	Failures []CouponFailureItem
}

func (e *CouponUnavailableError) Error() string {
	return fmt.Sprintf("%d 张优惠券不可用", len(e.Failures))
}
