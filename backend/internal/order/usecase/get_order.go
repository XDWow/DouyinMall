package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type GetOrderUseCase struct {
	orderRepo domain.OrderRepository
	log       logger.LoggerV1
}

func NewGetOrderUseCase(orderRepo domain.OrderRepository, log logger.LoggerV1) *GetOrderUseCase {
	return &GetOrderUseCase{
		orderRepo: orderRepo,
		log:       log,
	}
}

type GetOrderCmd struct {
	OrderID int64
}

func (uc *GetOrderUseCase) Execute(ctx context.Context, cmd GetOrderCmd) (*domain.Order, error) {
	if cmd.OrderID <= 0 {
		return nil, errors.New("invalid order id")
	}

	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	order, err := uc.orderRepo.FindByID(c, cmd.OrderID)
	if err != nil {
		uc.log.Error("查询订单失败", logger.Int64("orderID", cmd.OrderID), logger.Error(err))
		return nil, err
	}
	return &order, nil
}


