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
			Quantity:  -item.Quantity, // 棰勬墸鏄噺搴撳瓨锛屽彇璐?
		}
	}
	return uc.repo.ReserveStock(ctx, cmd.OperationID, changes, cmd.ExpireTime)
}

type ReserveStockCommand struct {
	OperationID string
	Changes     []StockItem
	ExpireTime  int64
}

// StockItem Quantity 涓烘鏁帮紝琛ㄧず璐拱鏁伴噺锛坲secase 灞傝礋璐ｈ浆涓鸿礋鏁颁紶缁?repo锛?
type StockItem struct {
	ProductID int64
	Quantity  int32
}


