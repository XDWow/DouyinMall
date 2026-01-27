package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
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
	Cursor int64 // 上一页最后的orderID，首次查询传0
	Limit  int32
}

type ListUserOrderResult struct {
	Orders     []*domain.Order
	NextCursor int64 // 下一页的cursor，0表示没有更多数据
}

func (uc *ListUserOrderUseCase) Execute(cmd ListUserOrderCmd) (*ListUserOrderResult, error) {
	// cmd不可信，参数校验
	if cmd.UserID <= 0 || cmd.Cursor < 0 || cmd.Limit <= 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	orders, nextCursor, err := uc.orderRepo.ListByUserID(ctx, cmd.UserID, cmd.Cursor, int(cmd.Limit))
	if err != nil {
		uc.log.Error("查询用户订单列表失败",
			logger.Int64("userID", cmd.UserID),
			logger.Int64("cursor", cmd.Cursor),
			logger.Int32("limit", cmd.Limit),
			logger.Error(err))
		return nil, err
	}
	return &ListUserOrderResult{
		Orders:     orders,
		NextCursor: nextCursor,
	}, nil
}
