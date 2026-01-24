package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/repository"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"time"
)

const OrderStatusChanged = "order.status.changed"

type ChangeOrderStatusUseCase struct {
	orderRepo  domain.OrderRepository
	outboxRepo domain.OutboxRepository
	producer   mq.SaramaProducer
	tx         TxManager
	log        logger.LoggerV1
}

func NewChangeOrderStatusUseCase(
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	tx TxManager,
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
	OrderID     int64
	OrderStatus domain.OrderStatus
}

func (uc *ChangeOrderStatusUseCase) Execute(cmd ChangeOrderStatusCmd) error {
	if cmd.OrderID <= 0 {
		return errors.New("订单ID无效")
	}
	order := domain.Order{
		ID:     cmd.OrderID,
		Status: cmd.OrderStatus,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// 需要执行结果，那就同步调用 rpc，否则异步MQ
	// 这个比较重要，所以通过 outbox 把要发的消息变为数据库事实（慢路径兜底，保证生产者消息不丢），并且和状态修改同一个DB事务提交
	err := uc.tx.WithTx(ctx, func(ctx context.Context) error {
		err := uc.orderRepo.UpdateStatus(ctx, &order)
		if err != nil {
			if errors.Is(err, repository.ErrRecordNotFound) {
				return errors.New("订单不存在或状态不能改变")
			}
			return err
		}
		event := domain.OrderStatusUpdateEvent{
			OrderID: order.ID,
			Status:  order.Status,
		}
		err = uc.outboxRepo.Add(ctx, OrderStatusChanged, event)
		if err != nil {
			uc.log.Error("保存outbox失败", logger.Error(err))
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// fast path
	go func() {
		c, can := context.WithTimeout(context.Background(), 3*time.Second)
		defer can()
		e := uc.producer.SendMessage(c, domain.OrderStatusUpdateEvent{order.ID, order.Status})
	if e != nil {
			uc.log.Error("订单状态变化事件发送失败", logger.Error(e))
			_, e = uc.outboxRepo.IncreaseRetry(c, order.ID)
			if e != nil {
				uc.log.Error("增加重试次数失败", logger.Error(e))
			}
			return
		}
		e = uc.outboxRepo.MarkSent(c, order.ID)
		if e != nil {
			uc.log.Error("修改发件箱状态为已发送，失败", logger.Error(e))
		}
	}()	

	return nil
}
