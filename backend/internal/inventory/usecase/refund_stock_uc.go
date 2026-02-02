package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type RefundStockCommand struct {
	OperationID string // 本次refund操作的ID（如order_123_refund），用于幂等检查和插入记录
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
		return errors.New("OperationID为空")
	}
	return uc.repo.RefundStock(ctx, cmd.OperationID)
}