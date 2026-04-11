package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type RefundStockCommand struct {
	OperationID string // 本次 refund 操作 ID（如 order_123_refund），用于幂等与落库
}

type RefundStockUseCase struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

func NewRefundStockUseCase(repo domain.InventoryRepository, l logger.LoggerV1) *RefundStockUseCase {
	return &RefundStockUseCase{repo: repo, l: l}
}

func (uc *RefundStockUseCase) Execute(ctx context.Context, cmd RefundStockCommand) error {
	if cmd.OperationID == "" {
		return errors.New("OperationID 为空")
	}
	return uc.repo.RefundStock(ctx, cmd.OperationID)
}

