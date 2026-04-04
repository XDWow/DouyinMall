package mq

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const TopicOrderStatusUpdate = "order_status_update"

// OrderStatusUpdateEvent 璁㈠崟鐘舵€佸彉鏇翠簨浠?
type OrderStatusUpdateEvent struct {
	OrderID int64       `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

type OrderStatus uint8

const (
	OrderStatusUnknown   OrderStatus = iota
	OrderStatusCreated               // 1: 寰呮敮浠?
	OrderStatusPaid                  // 2: 宸叉敮浠橈紙闇€瑕佺‘璁や紭鎯犲埜锛?
	OrderStatusShipped               // 4: 宸插彂璐?
	OrderStatusCompleted             // 5: 宸插畬鎴?
	OrderStatusCanceled              // 6: 宸插彇娑堬紙闇€瑕侀噴鏀句紭鎯犲埜锛?
	OrderStatusRefunded              // 7: 宸查€€娆撅紙闇€瑕侀€€杩樹紭鎯犲埜锛?
)

/*
OrderConsumer 璁㈠崟鐘舵€佸彉鏇存秷璐硅€?

鑱岃矗锛?
1. 鐩戝惉璁㈠崟鐘舵€佸彉鏇存秷鎭?
2. 鏍规嵁璁㈠崟鐘舵€佹墽琛屼紭鎯犲埜鎿嶄綔锛?
  - 鏀粯鎴愬姛(Paid) 鈫?纭浣跨敤浼樻儬鍒革紙Locked 鈫?Used锛?
  - 璁㈠崟鍙栨秷(Canceled) 鈫?閲婃斁浼樻儬鍒革紙Locked 鈫?Unused锛?
  - 璁㈠崟閫€娆?Refunded) 鈫?閫€杩樹紭鎯犲埜锛圲sed 鈫?Unused锛?

璁捐鍘熷垯锛?
- 骞傜瓑鎬э細鍒╃敤UpdateStatusByOrderID鐨勬潯浠舵洿鏂颁繚璇佸箓绛?
- 瀹归敊鎬э細浣跨敤鏈湴閲嶈瘯 + 缁熶竴ACK锛坰aramax.Handler鍐呯疆3娆￠噸璇曪級
- 瑙ｈ€︼細鍙礋璐ｆ秷鎭矾鐢憋紝涓氬姟閫昏緫鍦║seCase灞?
*/
type OrderConsumer struct {
	client    sarama.Client
	commitUC  *usecase.CommitCouponUseCase  // 纭浣跨敤
	releaseUC *usecase.ReleaseCouponUseCase // 閲婃斁锛堝彇娑堬級
	refundUC  *usecase.RefundCouponUseCase  // 閫€杩橈紙閫€娆撅級
	logger    logger.LoggerV1
}

func NewOrderConsumer(
	client sarama.Client,
	commitUC *usecase.CommitCouponUseCase,
	releaseUC *usecase.ReleaseCouponUseCase,
	refundUC *usecase.RefundCouponUseCase,
	l logger.LoggerV1,
) *OrderConsumer {
	return &OrderConsumer{
		client:    client,
		commitUC:  commitUC,
		releaseUC: releaseUC,
		refundUC:  refundUC,
		logger:    l,
	}
}

func (c *OrderConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("coupon-consumer", c.client)
	if err != nil {
		return err
	}

	go func() {
		for {
			err := cg.Consume(
				context.Background(),
				[]string{TopicOrderStatusUpdate},
				saramax.NewHandler[OrderStatusUpdateEvent](c.logger, c.Consume),
			)
			if err != nil {
				c.logger.Error("浼樻儬鍒告秷璐硅€呭紓甯搁€€鍑?, logger.Error(err))
				// 鐭殏绛夊緟鍚庨噸璇曪紝閬垮厤鐤媯閲嶈繛
				time.Sleep(time.Second)
			}
		}
	}()

	c.logger.Info("OrderConsumer宸插惎鍔?,
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "coupon-consumer"))

	return nil
}

// Consume 娑堟伅璺敱锛氭牴鎹姸鎬佸垎鍙戝埌涓嶅悓澶勭悊鏂规硶
// 杩斿洖nil琛ㄧずACK锛岃繑鍥瀍rror浼氳Е鍙憇aramax.Handler鐨勯噸璇曢€昏緫锛堟渶澶?娆★級
func (c *OrderConsumer) Consume(msg *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("鏀跺埌璁㈠崟鐘舵€佸彉鏇存秷鎭?,
		logger.Int64("orderID", evt.OrderID),
		logger.Int("status", int(evt.Status)),
		logger.String("topic", msg.Topic),
		logger.Int64("partition", int64(msg.Partition)),
		logger.Int64("offset", msg.Offset))

	switch evt.Status {
	case OrderStatusPaid:
		return c.handlePaid(ctx, evt)
	case OrderStatusCanceled:
		return c.handleCanceled(ctx, evt)
	case OrderStatusRefunded:
		return c.handleRefunded(ctx, evt)
	default:
		// 鍏朵粬鐘舵€佷笉闇€瑕佸鐞嗭紝鐩存帴ACK
		return nil
	}
}

// 璁㈠崟鏀粯鎴愬姛 鈫?纭浣跨敤浼樻儬鍒革紙Locked 鈫?Used锛?
func (c *OrderConsumer) handlePaid(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("纭浣跨敤浼樻儬鍒稿け璐?,
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 杩斿洖error瑙﹀彂閲嶈瘯
	}

	c.logger.Info("纭浣跨敤浼樻儬鍒告垚鍔?,
		logger.Int64("orderID", evt.OrderID))
	return nil
}

// 璁㈠崟鍙栨秷 鈫?閲婃斁浼樻儬鍒革紙Locked 鈫?Unused锛?
func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("閲婃斁浼樻儬鍒稿け璐?,
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 杩斿洖error瑙﹀彂閲嶈瘯
	}

	c.logger.Info("閲婃斁浼樻儬鍒告垚鍔?,
		logger.Int64("orderID", evt.OrderID))
	return nil
}

// 璁㈠崟閫€娆?鈫?閫€杩樹紭鎯犲埜锛圲sed 鈫?Unused锛?
func (c *OrderConsumer) handleRefunded(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.refundUC.Execute(ctx, usecase.RefundCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("閫€杩樹紭鎯犲埜澶辫触",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 杩斿洖error瑙﹀彂閲嶈瘯
	}

	c.logger.Info("閫€杩樹紭鎯犲埜鎴愬姛",
		logger.Int64("orderID", evt.OrderID))
	return nil
}


