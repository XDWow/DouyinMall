package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrPriceChanged        = errors.New("price has changed, please re-confirm")
	ErrInsufficientStock   = errors.New("insufficient stock")
	ErrOrderCreateFailed   = errors.New("failed to create order")
	ErrPaymentCreateFailed = errors.New("failed to create payment")
	ErrOrderNotPayable     = errors.New("order is not payable")
	ErrOrderExpired        = errors.New("order has expired")
	ErrOrderForbidden      = errors.New("order does not belong to current user")
)

type UnavailableItem struct {
	ProductID int64
	SKUID     int64
	Name      string
	Reason    string
}

type UnavailableItemsError struct {
	Items []UnavailableItem
}

func (e *UnavailableItemsError) Error() string {
	return fmt.Sprintf("%d items are unavailable", len(e.Items))
}

type InsufficientStockItem struct {
	ProductID int64
	Name      string
	Requested int64
	Available int64
}

type InsufficientStockError struct {
	Items []InsufficientStockItem
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("%d items have insufficient stock", len(e.Items))
}

type CouponFailureItem struct {
	CouponID int64
	Reason   string
}

type CouponUnavailableError struct {
	Failures []CouponFailureItem
}

func (e *CouponUnavailableError) Error() string {
	return fmt.Sprintf("%d coupons are unavailable", len(e.Failures))
}
