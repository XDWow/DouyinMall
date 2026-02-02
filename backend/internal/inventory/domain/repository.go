package domain

import "context"

type InventoryRepository interface {
	GetInventory(ctx context.Context, productID []int64) ([]Inventory, error)

	// ReserveStock 预扣库存（Redis）
	ReserveStock(ctx context.Context, reserveID string, changes []StockChange, expireTime int64) error

	// CommitStock 确认扣减（DB）
	CommitStock(ctx context.Context, operationID string, changes []StockChange) error

	// ReleaseStock 释放预扣（从Redis预扣记录读取商品信息）
	ReleaseStock(ctx context.Context, reserveID string) error

	// RefundStock 退款（从DB的commit记录读取商品信息）
	RefundStock(ctx context.Context, operationID string) error

	// AdjustStock 人工调整库存
	AdjustStock(ctx context.Context, operationID string, reason string, changes []StockChange) error
}
