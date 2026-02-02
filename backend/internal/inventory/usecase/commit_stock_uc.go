package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// 1. 用户支付成功（钱到账），订单状态：已支付
// 2. 系统尝试 commitStock（DB）
// 3. commit 成功
// → 订单成功
// 4. commit 失败
// → 订单失败
// → 失败就自动退款
type CommitStockCommand struct {
	OperationID string
	Changes     []domain.StockChange
}

type CommitStockUseCase struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

func NewCommitStockUseCase(repo domain.InventoryRepository, l logger.LoggerV1) *CommitStockUseCase {
	return &CommitStockUseCase{repo: repo, l: l}
}

func (uc *CommitStockUseCase) Execute(ctx context.Context, cmd CommitStockCommand) error {
	if cmd.OperationID == "" {
		return errors.New("OperationID为空")
	}
	if cmd.Changes == nil {
		return errors.New("Changes为空")
	}
	return uc.repo.CommitStock(ctx, cmd.OperationID, cmd.Changes)
}
