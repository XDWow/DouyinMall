package usecase

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

type GetResultUseCase struct {
	requestRepo domain.RequestRepository
	cache       domain.Cache
}

func NewGetResultUseCase(requestRepo domain.RequestRepository, cache domain.Cache) *GetResultUseCase {
	return &GetResultUseCase{requestRepo: requestRepo, cache: cache}
}

func (uc *GetResultUseCase) Execute(ctx context.Context, requestNo string) (*domain.Result, error) {
	result, err := uc.cache.GetResult(ctx, requestNo)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	request, err := uc.requestRepo.FindByRequestNo(ctx, requestNo)
	if err != nil {
		return nil, err
	}
	result = &domain.Result{
		RequestNo:  request.RequestNo,
		Status:     request.Status,
		OrderID:    request.OrderID,
		FailReason: request.FailReason,
	}
	_ = uc.cache.SetResult(ctx, *result)
	return result, nil
}


