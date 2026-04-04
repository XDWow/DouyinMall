package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

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
		return errors.New("operation id is empty")
	}
	if cmd.Changes == nil {
		return errors.New("changes is empty")
	}
	return uc.repo.CommitStock(ctx, cmd.OperationID, cmd.Changes)
}


