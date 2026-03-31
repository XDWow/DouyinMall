package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ReserveStockUsecase struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

func NewReserveStockUsecase(repo domain.InventoryRepository, l logger.LoggerV1) *ReserveStockUsecase {
	return &ReserveStockUsecase{repo, l}
}

func (uc *ReserveStockUsecase) Execute(ctx context.Context, cmd ReserveStockCommand) error {
	if cmd.OperationID == "" {
		return errors.New("OperationID is empty")
	}
	changes := make([]domain.StockChange, len(cmd.Changes))
	for i, item := range cmd.Changes {
		changes[i] = domain.StockChange{
			ProductID: item.ProductID,
			Quantity:  -item.Quantity, // 预扣是减库存，取负
		}
	}
	return uc.repo.ReserveStock(ctx, cmd.OperationID, changes, cmd.ExpireTime)
}

type ReserveStockCommand struct {
	OperationID string
	Changes     []StockItem
	ExpireTime  int64
}

// StockItem Quantity 为正数，表示购买数量（usecase 层负责转为负数传给 repo）
type StockItem struct {
	ProductID int64
	Quantity  int32
}
