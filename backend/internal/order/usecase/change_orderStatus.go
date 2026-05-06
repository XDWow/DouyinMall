package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ChangeOrderStatusUseCase struct {
	orderRepo  domain.OrderRepository
	outboxRepo domain.OutboxRepository
	producer   mq.OrderStatusProducer
	tx         domain.TxManager
	log        logger.LoggerV1
}

func NewChangeOrderStatusUseCase(
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.OrderStatusProducer,
	tx domain.TxManager,
	log logger.LoggerV1,
) *ChangeOrderStatusUseCase {
	return &ChangeOrderStatusUseCase{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		producer:   producer,
		tx:         tx,
		log:        log,
	}
}

type ChangeOrderStatusCmd struct {
	OrderID int64
	Action  domain.OrderAction
}

type ChangeOrderStatusResult struct {
	Changed bool
}

func (uc *ChangeOrderStatusUseCase) ChangeOrderStatus(ctx context.Context, orderID int64, action domain.OrderAction) error {
	_, err := uc.Execute(ctx, ChangeOrderStatusCmd{
		OrderID: orderID,
		Action:  action,
	})
	return err
}

func (uc *ChangeOrderStatusUseCase) Execute(ctx context.Context, cmd ChangeOrderStatusCmd) (ChangeOrderStatusResult, error) {
	if cmd.OrderID <= 0 {
		return ChangeOrderStatusResult{}, errors.New("invalid order id")
	}
	if cmd.Action == domain.OrderActionUnknown {
		return ChangeOrderStatusResult{}, errors.New("invalid order action")
	}

	fullOrder, err := uc.orderRepo.FindByID(ctx, cmd.OrderID)
	if err != nil {
		return ChangeOrderStatusResult{}, err
	}

	fromStatus := fullOrder.Status
	if err := applyOrderAction(&fullOrder, cmd.Action); err != nil {
		if errors.Is(err, domain.ErrOrderStatusUnchanged) {
			return ChangeOrderStatusResult{Changed: false}, nil
		}
		return ChangeOrderStatusResult{}, err
	}
	if fromStatus == fullOrder.Status {
		return ChangeOrderStatusResult{Changed: false}, nil
	}

	event := domain.BuildOrderStatusUpdateEvent(&fullOrder)

	var outboxID int64
	err = uc.tx.Tx(ctx, func(ctx context.Context) error {
		if err := uc.orderRepo.UpdateStatus(ctx, fullOrder.ID, fromStatus, fullOrder.Status); err != nil {
			if errors.Is(err, domain.ErrRecordNotFound) {
				return domain.ErrInvalidStatusTransition
			}
			return err
		}
		var addErr error
		outboxID, addErr = uc.outboxRepo.Add(ctx, domain.EventTypeOrderStatusChanged, event)
		return addErr
	})
	if err != nil {
		return ChangeOrderStatusResult{}, err
	}

	go func(outboxID int64, evt domain.OrderStatusUpdateEvent) {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if sendErr := uc.producer.SendMessage(c, evt); sendErr != nil {
			uc.log.Error("发送订单状态变更事件失败",
				logger.Error(sendErr),
				logger.Int64("orderID", evt.OrderID),
				logger.Int64("outboxID", outboxID))
			return
		}

		if markErr := uc.outboxRepo.MarkSent(c, outboxID); markErr != nil {
			uc.log.Error("标记 outbox 已发送，失败",
				logger.Error(markErr),
				logger.Int64("orderID", evt.OrderID),
				logger.Int64("outboxID", outboxID))
		}
	}(outboxID, event)

	return ChangeOrderStatusResult{Changed: true}, nil
}

func applyOrderAction(order *domain.Order, action domain.OrderAction) error {
	switch action {
	case domain.OrderActionPay:
		return order.Pay()
	case domain.OrderActionShip:
		return order.Ship()
	case domain.OrderActionComplete:
		return order.Complete()
	case domain.OrderActionCancel:
		return order.Cancel()
	case domain.OrderActionRefund:
		return order.Refund()
	default:
		return domain.ErrInvalidStatusTransition
	}
}
