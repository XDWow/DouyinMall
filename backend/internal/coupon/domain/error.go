package domain

import "errors"

var (
	ErrCouponNotFound         = errors.New("coupon not found")
	ErrCouponNotAvailable     = errors.New("coupon not available")
	ErrCouponExpired          = errors.New("coupon expired")
	ErrCouponAlreadyUsed      = errors.New("coupon already used")
	ErrCouponAlreadyLocked    = errors.New("coupon already locked")
	ErrCouponNotLocked        = errors.New("coupon not locked")
	ErrCouponNotOwned         = errors.New("coupon not owned by user")
	ErrThresholdNotMet        = errors.New("order amount does not meet coupon threshold")
	ErrOrderNotApplicable     = errors.New("coupon is not applicable to current order")
	ErrDuplicateOperation     = errors.New("duplicate coupon operation")
	ErrOperationNotFound      = errors.New("coupon operation not found")
	ErrIssueLimitExceeded     = errors.New("coupon issue limit exceeded")
	ErrCouponLimitExceeded    = errors.New("user coupon limit exceeded")
	ErrTemplateStockOut       = errors.New("coupon template stock out")
	ErrCouponTemplateNotFound = errors.New("coupon template not found")
	ErrCouponCannotIssue      = errors.New("coupon cannot be issued")
)
