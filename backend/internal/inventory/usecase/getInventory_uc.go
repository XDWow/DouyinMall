package usecase

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type GetInventoryUseCase struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

// 业务中单个和批量查询，只需一个 usecase 逻辑，避免大量重复代码
type GetInventoryQuery struct {
	ProductID []int64
}

func NewGetInventoryUseCase(repo domain.InventoryRepository, l logger.LoggerV1) *GetInventoryUseCase {
	return &GetInventoryUseCase{repo, l}
}

func (uc *GetInventoryUseCase) Execute(ctx context.Context, q GetInventoryQuery) ([]domain.Inventory, error) {
	var IDs []int64
	for _, v := range q.ProductID {
		if v > 0 {
			IDs = append(IDs, v)
		} else {
			uc.l.Error("商品id无效", logger.Int64("ID", v))
		}
	}

	return uc.repo.GetInventory(ctx, IDs)
}
