package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/aftersale/domain"
)

type GetAfterSaleRequestUseCase struct {
	repo domain.RequestRepository
}

type GetAfterSaleRequestCmd struct {
	RequestNo string
}

func NewGetAfterSaleRequestUseCase(repo domain.RequestRepository) *GetAfterSaleRequestUseCase {
	return &GetAfterSaleRequestUseCase{repo: repo}
}

func (uc *GetAfterSaleRequestUseCase) Execute(ctx context.Context, cmd GetAfterSaleRequestCmd) (*domain.Request, error) {
	requestNo := strings.TrimSpace(cmd.RequestNo)
	if requestNo == "" {
		return nil, fmt.Errorf("request_no is required")
	}
	return uc.repo.GetByRequestNo(ctx, requestNo)
}
