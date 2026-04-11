package domain

import (
	"errors"
	"time"

	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
)

type Order struct {
	ID         int64
	UserID     int64
	Remark     string
	Addr       Address
	Status     OrderStatus
	OrderKind  string
	ActivityID int64
	CreatedAt  time.Time
	ExpireAt   time.Time

	TotalAmount    Amount
	PayableAmount  Amount
	DiscountAmount Amount

	OrderItems []OrderItem
}

type OrderItem struct {
	ProductID        int64
	SKUID            int64
	Quantity         int64
	SnapshotPrice    int64
	SnapshotCurrency string
	Price            int64
}

type Address struct {
	Street  string
	City    string
	State   string
	Country string
	Zipcode string
	Phone   string
}

type Amount struct {
	Currency string
	Total    int64
}

const (
	OrderKindDirectBuy = "DIRECT_BUY"
	OrderKindCart      = "CART"
	OrderKindSeckill   = "SECKILL"
)

type OrderStatus = orderv1.OrderStatus
type OrderAction = orderv1.ChangeOrderAction

const (
	OrderStatusUnknown   = orderv1.OrderStatus_ORDER_STATUS_UNKNOWN
	OrderStatusCreated   = orderv1.OrderStatus_ORDER_STATUS_CREATED
	OrderStatusPaid      = orderv1.OrderStatus_ORDER_STATUS_PAID
	OrderStatusShipped   = orderv1.OrderStatus_ORDER_STATUS_SHIPPED
	OrderStatusCompleted = orderv1.OrderStatus_ORDER_STATUS_COMPLETED
	OrderStatusCanceled  = orderv1.OrderStatus_ORDER_STATUS_CANCELED
	OrderStatusRefunded  = orderv1.OrderStatus_ORDER_STATUS_REFUNDED
)

const (
	OrderActionUnknown  = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_UNKNOWN
	OrderActionPay      = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_PAY
	OrderActionShip     = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_SHIP
	OrderActionComplete = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_COMPLETE
	OrderActionCancel   = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_CANCEL
	OrderActionRefund   = orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_REFUND
)

var ErrInvalidStatusTransition = errors.New("状态转移无效")
var ErrOrderStatusUnchanged = errors.New("订单状态不可变")

func (o *Order) Pay() error {
	if o.Status == OrderStatusPaid {
		return ErrOrderStatusUnchanged
	}
	if o.Status != OrderStatusCreated {
		return ErrInvalidStatusTransition
	}

	o.Status = OrderStatusPaid
	return nil
}

func (o *Order) Ship() error {
	if o.Status == OrderStatusShipped {
		return ErrOrderStatusUnchanged
	}
	if o.Status != OrderStatusPaid {
		return ErrInvalidStatusTransition
	}

	o.Status = OrderStatusShipped
	return nil
}

func (o *Order) Complete() error {
	if o.Status == OrderStatusCompleted {
		return ErrOrderStatusUnchanged
	}
	if o.Status != OrderStatusShipped {
		return ErrInvalidStatusTransition
	}

	o.Status = OrderStatusCompleted
	return nil
}

func (o *Order) Cancel() error {
	if o.Status == OrderStatusCanceled {
		return ErrOrderStatusUnchanged
	}
	if o.Status != OrderStatusCreated {
		return ErrInvalidStatusTransition
	}

	o.Status = OrderStatusCanceled
	return nil
}

func (o *Order) Refund() error {
	if o.Status == OrderStatusRefunded {
		return ErrOrderStatusUnchanged
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusShipped {
		return ErrInvalidStatusTransition
	}

	o.Status = OrderStatusRefunded
	return nil
}
