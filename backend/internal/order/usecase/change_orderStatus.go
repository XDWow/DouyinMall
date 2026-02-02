package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
)

const OrderStatusChanged = "order.status.changed"

type ChangeOrderStatusUseCase struct {
	orderRepo    domain.OrderRepository
	outboxRepo   domain.OutboxRepository
	producer     mq.SaramaProducer
	tx           domain.TxManager
	log          logger.LoggerV1
	inventoryCli inventoryservice.Client
}

func NewChangeOrderStatusUseCase(
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	tx domain.TxManager,
	log logger.LoggerV1,
	inventoryCli inventoryservice.Client,
) *ChangeOrderStatusUseCase {
	return &ChangeOrderStatusUseCase{
		orderRepo:    orderRepo,
		outboxRepo:   outboxRepo,
		producer:     producer,
		tx:           tx,
		log:          log,
		inventoryCli: inventoryCli,
	}
}

type ChangeOrderStatusCmd struct {
	OrderID     int64
	OrderStatus domain.OrderStatus
}

func (uc *ChangeOrderStatusUseCase) Execute(ctx context.Context, cmd ChangeOrderStatusCmd) error {
	if cmd.OrderID <= 0 {
		return errors.New("订单ID无效")
	}

	return uc.updateStatusAndPublish(ctx, cmd.OrderID, cmd.OrderStatus)
}

/*
// 处理支付成功的同步流程：支付成功 → CommitStock → 成功/失败 → 订单成功/触发退款
func (uc *ChangeOrderStatusUseCase) handlePaymentSuccess(ctx context.Context, orderID int64) error {
	// 1. 查询订单详情（获取Items信息用于CommitStock）
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		uc.log.Error("查询订单失败", logger.Error(err), logger.Int64("orderID", orderID))
		return err
	}

	// 2. 同步调用库存服务CommitStock
	items := make([]*inventoryv1.StockItem, len(order.OrderItems))
	for i, item := range order.OrderItems {
		items[i] = &inventoryv1.StockItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
		}
	}

	operationID := buildOrderOperationID(orderID, "commit")
	resp, err := uc.inventoryCli.CommitStock(ctx, &inventoryv1.CommitStockReq{
		OperationId: operationID,
		Items:       items,
	})

	// 3. 根据CommitStock结果决定订单状态
	if err != nil || resp.StatusCode != 0 {
		// CommitStock失败 → 订单失败 → 后续需触发退款
		uc.log.Error("CommitStock失败，订单标记为失败",
			logger.Error(err),
			logger.Int64("orderID", orderID),
			logger.String("resp_msg", resp.GetStatusMsg()))

		// TODO: 这里应该触发退款流程（调用支付服务Refund接口）
		// 暂时先标记订单为取消状态
		return uc.updateStatusAndPublish(ctx, orderID, domain.OrderStatusCanceled)
	}

	// 4. CommitStock成功 → 订单成功
	uc.log.Info("CommitStock成功，订单支付成功", logger.Int64("orderID", orderID))
	return uc.updateStatusAndPublish(ctx, orderID, domain.OrderStatusPaid)
}
*/

// 构造订单相关的operationID：order_{orderID}_{action}
// 用于库存操作的幂等性标识，与库存服务保持一致
func buildOrderOperationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

// 异步：更新订单状态并发布MQ事件
func (uc *ChangeOrderStatusUseCase) updateStatusAndPublish(ctx context.Context, orderID int64, status domain.OrderStatus) error {
	order := domain.Order{
		ID:     orderID,
		Status: status,
	}

	// CommitStock需要Items，先查询订单
	var eventItems []domain.OrderEventItem
	if status == domain.OrderStatusPaid {
		fullOrder, err := uc.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			uc.log.Error("查询订单失败", logger.Error(err), logger.Int64("orderID", orderID))
			return err
		}
		eventItems = make([]domain.OrderEventItem, len(fullOrder.OrderItems))
		for i, item := range fullOrder.OrderItems {
			eventItems[i] = domain.OrderEventItem{
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
			}
		}
	}

	// 必须要拿到执行结果，那就同步调用 rpc，否则异步MQ
	// 这个比较重要，所以通过 outbox 把要发的消息变为数据库事实（慢路径兜底，保证生产者消息不丢），并且和状态修改同一个DB事务提交
	err := uc.tx.Tx(ctx, func(ctx context.Context) error {
		err := uc.orderRepo.UpdateStatus(ctx, &order)
		if err != nil {
			if errors.Is(err, domain.ErrRecordNotFound) {
				return errors.New("订单不存在或状态不能改变")
			}
			return err
		}
		event := domain.OrderStatusUpdateEvent{
			OrderID: order.ID,
			Status:  order.Status,
			Items:   eventItems, // 状态为Paid时有值，其他状态为nil
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
		e := uc.producer.SendMessage(c, domain.OrderStatusUpdateEvent{
			OrderID: order.ID,
			Status:  order.Status,
			Items:   eventItems,
		})
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
