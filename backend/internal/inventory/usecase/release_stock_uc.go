package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ReleaseStockCommand struct {
	OperationID string // 本次release操作的ID（如order_123_release）
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
		return errors.New("OperationID为空")
	}
	return uc.repo.ReleaseStock(ctx, cmd.OperationID)
}
