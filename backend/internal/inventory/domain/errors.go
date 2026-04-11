package domain

import (
	"errors"
	"fmt"
)

var ErrDuplicateOperation = errors.New("操作已执行过")

var ErrProductNotFound = errors.New("商品不存在")

var ErrInsufficientStock = errors.New("库存不足") // CommitStock DB 层用

// InsufficientStockItem 单个商品库存不足明细
type InsufficientStockItem struct {
	ProductID int64
	Requested int64 // 请求预扣数量
	Available int64 // 可用库存 = 实际库存 - Redis 已预扣库存
}

// InsufficientStockError 预扣库存失败，附带所有不足商品明细
type InsufficientStockError struct {
	Items []InsufficientStockItem
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("%d 件商品库存不足", len(e.Items))
}
