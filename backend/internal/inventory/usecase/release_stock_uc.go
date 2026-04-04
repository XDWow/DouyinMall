package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ReleaseStockCommand struct {
	OperationID string // 鏈release鎿嶄綔鐨処D锛堝order_123_release锛?
}

type ReleaseStockUseCase struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

func NewReleaseStockUseCase(repo domain.InventoryRepository, l logger.LoggerV1) *ReleaseStockUseCase {
	return &ReleaseStockUseCase{repo: repo, l: l}
}

func (uc *ReleaseStockUseCase) Execute(ctx context.Context, cmd ReleaseStockCommand) error {
	if cmd.OperationID == "" {
		return errors.New("OperationID涓虹┖")
	}
	return uc.repo.ReleaseStock(ctx, cmd.OperationID)
}


