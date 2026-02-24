package domain

import "errors"

var (
	ErrCouponNotFound         = errors.New("优惠券不存在")
	ErrCouponNotAvailable     = errors.New("优惠券不可用")
	ErrCouponExpired          = errors.New("优惠券已过期")
	ErrCouponAlreadyUsed      = errors.New("优惠券已使用")
	ErrCouponAlreadyLocked    = errors.New("优惠券已被其他订单锁定")
	ErrCouponNotLocked        = errors.New("优惠券未锁定")
	ErrCouponNotOwned         = errors.New("优惠券不属于当前用户")
	ErrThresholdNotMet        = errors.New("未达到使用门槛")
	ErrOrderNotApplicable     = errors.New("订单中无适用商品")
	ErrDuplicateOperation     = errors.New("重复操作")
	ErrOperationNotFound      = errors.New("操作记录不存在")
	ErrIssueLimitExceeded     = errors.New("已达领取上限")
	ErrCouponLimitExceeded    = errors.New("已达领取上限")
	ErrTemplateStockOut       = errors.New("优惠券已发完")
	ErrCouponTemplateNotFound = errors.New("优惠券模板不存在")
	ErrCouponCannotIssue      = errors.New("优惠券不可发放")
)
