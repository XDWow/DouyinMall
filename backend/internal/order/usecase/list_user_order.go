package usecase

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"time"
)

type ListUserOrderUseCase struct {
	orderRepo domain.OrderRepository
	log       logger.LoggerV1
}

func NewListUserOrderUseCase(
	orderRepo domain.OrderRepository,
	log logger.LoggerV1,
) *ListUserOrderUseCase {
	return &ListUserOrderUseCase{
		orderRepo: orderRepo,
		log:       log,
	}
}

type ListUserOrderCmd struct {
	UserID int64
	Offset int
	Limit  int
}

func (uc *ListUserOrderUseCase) Execute(cmd ListUserOrderCmd) ([]domain.Order, error) {
	// cmd不可信，参数校验
	if cmd.UserID <= 0 || cmd.Offset < 0 || cmd.Limit <= 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	orders, err := uc.orderRepo.ListByUserID(ctx, cmd.UserID, cmd.Offset, cmd.Limit)
	if err != nil {
		uc.log.Error("查询用户订单列表失败",
			logger.Int64("userID", cmd.UserID),
			logger.Int("offset", cmd.Offset),
			logger.Int("limit", cmd.Limit),
			logger.Error(err))
		return nil, err
	}
	return orders, nil
}