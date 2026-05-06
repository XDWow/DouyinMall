package domain

import "errors"

var (
	ErrRecordNotFound          = errors.New("record not found")
	ErrInvalidUser             = errors.New("invalid user")
	ErrEmptyOrderItems         = errors.New("order items are empty")
	ErrSeckillActivityRequired = errors.New("seckill activity id is required")
	ErrDuplicateOrder          = errors.New("duplicate order")
)

const (
	BizStatusCreateOrderInvalidUser             int32 = 4001
	BizStatusCreateOrderEmptyItems              int32 = 4002
	BizStatusCreateOrderSeckillActivityRequired int32 = 4003
	BizStatusGetOrderNotFound                   int32 = 4404
)

func IsCreateOrderBizStatus(code int32) bool {
	switch code {
	case BizStatusCreateOrderInvalidUser,
		BizStatusCreateOrderEmptyItems,
		BizStatusCreateOrderSeckillActivityRequired:
		return true
	default:
		return false
	}
}

func IsGetOrderNotFoundBizStatus(code int32) bool {
	return code == BizStatusGetOrderNotFound
}
