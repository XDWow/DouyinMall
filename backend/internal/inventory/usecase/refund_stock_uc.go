package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type RefundStockCommand struct {
	OperationID string // 鏈refund鎿嶄綔鐨処D锛堝order_123_refund锛夛紝鐢ㄤ簬骞傜瓑妫€鏌ュ拰鎻掑叆璁板綍
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
		return errors.New("OperationID涓虹┖")
	}
	return uc.repo.RefundStock(ctx, cmd.OperationID)
}

