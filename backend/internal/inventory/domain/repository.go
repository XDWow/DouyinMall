package domain

import "context"

type InventoryRepository interface {
	GetInventory(ctx context.Context, productID []int64) ([]Inventory, error)
	CommitStock(ctx context.Context, operationID string, changes []StockChange) error
	RefundStock(ctx context.Context, operationID string) error
	AdjustStock(ctx context.Context, operationID string, reason string, changes []StockChange) error
}
