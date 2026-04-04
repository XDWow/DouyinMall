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

// 涓氬姟涓崟涓拰鎵归噺鏌ヨ锛屽彧闇€涓€涓?usecase 閫昏緫锛岄伩鍏嶅ぇ閲忛噸澶嶄唬鐮?
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
			uc.l.Error("鍟嗗搧id鏃犳晥", logger.Int64("ID", v))
		}
	}

	return uc.repo.GetInventory(ctx, IDs)
}


