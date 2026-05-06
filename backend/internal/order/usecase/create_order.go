package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CreateOrderUseCase struct {
	repo       domain.OrderRepository
	delayQueue domain.DelayQueue
	log        logger.LoggerV1
}

type CreateOrderCmd struct {
	OrderID       int64
	UserID        int64
	Currency      string
	Remark        string
	Address       domain.Address
	OrderKind     string
	ActivityID    int64
	PayableAmount int64
	Items         []domain.OrderItem
}

func NewCreateOrderUseCase(repo domain.OrderRepository, delayQueue domain.DelayQueue, log logger.LoggerV1) *CreateOrderUseCase {
	return &CreateOrderUseCase{
		repo:       repo,
		delayQueue: delayQueue,
		log:        log,
	}
}

func (uc *CreateOrderUseCase) Execute(ctx context.Context, cmd CreateOrderCmd) (int64, error) {
	if cmd.UserID <= 0 {
		return 0, domain.ErrInvalidUser
	}
	if len(cmd.Items) == 0 {
		return 0, domain.ErrEmptyOrderItems
	}
	if normalizeOrderKind(cmd.OrderKind) == domain.OrderKindSeckill && cmd.ActivityID <= 0 {
		return 0, domain.ErrSeckillActivityRequired
	}

	order := toDomainOrder(cmd)
	if err := uc.repo.Save(ctx, &order); err != nil {
		if errors.Is(err, domain.ErrDuplicateOrder) {
			existing, findErr := uc.repo.FindByID(ctx, cmd.OrderID)
			if findErr != nil {
				uc.log.Warn("重复创建订单时查询已存在订单失败",
					logger.Error(findErr),
					logger.Int64("orderID", cmd.OrderID))
				return 0, err
			}
			if sameCreateOrderIntent(existing, cmd) {
				return existing.ID, nil
			}
		}
		uc.log.Error("保存订单失败", logger.Error(err))
		return 0, err
	}

	go func(orderID int64, expireAt time.Time) {
		if uc.delayQueue == nil {
			return
		}

		queueCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := uc.delayQueue.Enqueue(queueCtx, orderID, expireAt); err != nil {
			uc.log.Warn("订单超时任务入队失败",
				logger.Error(err),
				logger.Int64("orderID", orderID))
		}
	}(order.ID, order.ExpireAt)

	return order.ID, nil
}

func toDomainOrder(cmd CreateOrderCmd) domain.Order {
	total := int64(0)
	for _, item := range cmd.Items {
		total += item.Price * item.Quantity
	}

	payable := cmd.PayableAmount
	if payable <= 0 {
		payable = total
	}
	discount := total - payable
	if discount < 0 {
		discount = 0
	}

	return domain.Order{
		ID:         cmd.OrderID,
		UserID:     cmd.UserID,
		Remark:     cmd.Remark,
		Status:     domain.OrderStatusCreated,
		OrderKind:  normalizeOrderKind(cmd.OrderKind),
		ActivityID: cmd.ActivityID,
		TotalAmount: domain.Amount{
			Currency: cmd.Currency,
			Total:    total,
		},
		PayableAmount: domain.Amount{
			Currency: cmd.Currency,
			Total:    payable,
		},
		DiscountAmount: domain.Amount{
			Currency: cmd.Currency,
			Total:    discount,
		},
		Addr:       cmd.Address,
		OrderItems: cmd.Items,
		ExpireAt:   time.Now().Add(30 * time.Minute),
	}
}

func normalizeOrderKind(orderKind string) string {
	if orderKind == "" {
		return domain.OrderKindDirectBuy
	}
	return orderKind
}

func sameCreateOrderIntent(existing domain.Order, cmd CreateOrderCmd) bool {
	return existing.ID == cmd.OrderID &&
		existing.UserID == cmd.UserID &&
		existing.ActivityID == cmd.ActivityID &&
		existing.OrderKind == normalizeOrderKind(cmd.OrderKind)
}
