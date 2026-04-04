package usecase

import (
	"context"
	"errors"
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
	Cursor int64 // 涓婁竴椤垫渶鍚庣殑orderID锛岄娆℃煡璇紶0
	Limit  int32
}

type ListUserOrderResult struct {
	Orders     []*domain.Order
	NextCursor int64 // 涓嬩竴椤电殑cursor锛?琛ㄧず娌℃湁鏇村鏁版嵁
}

func (uc *ListUserOrderUseCase) Execute(cmd ListUserOrderCmd) (*ListUserOrderResult, error) {
	// cmd涓嶅彲淇★紝鍙傛暟鏍￠獙
	if cmd.UserID <= 0 || cmd.Cursor < 0 || cmd.Limit <= 0 {
		return nil, errors.New("invalid list order query")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	orders, nextCursor, err := uc.orderRepo.ListByUserID(ctx, cmd.UserID, cmd.Cursor, int(cmd.Limit))
	if err != nil {
		uc.log.Error("鏌ヨ鐢ㄦ埛璁㈠崟鍒楄〃澶辫触",
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


