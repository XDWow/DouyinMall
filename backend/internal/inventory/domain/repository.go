package domain

import "context"

type InventoryRepository interface {
	GetInventory(ctx context.Context, productID []int64) ([]Inventory, error)
	ReserveStock(ctx context.Context, reserveID string, changes []StockChange, expireTime int64) error
	CommitStock(ctx context.Context, operationID string, changes []StockChange) error
	ReleaseStock(ctx context.Context, reserveID string) error
	RefundStock(ctx context.Context, operationID string) error
	AdjustStock(ctx context.Context, operationID string, reason string, changes []StockChange) error
}
