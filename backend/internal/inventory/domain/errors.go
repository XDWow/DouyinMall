package domain

import "errors"

var ErrDuplicateOperation = errors.New("操作已执行过")

var ErrInsufficientStock = errors.New("库存不足")
