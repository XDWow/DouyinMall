package domain

import "time"

type Order struct {
	ID         int64
	UserID     int64
	Phone      string
	Status     OrderStatus
	Amt        Amount
	Addr       Address
	CreatedAt  time.Time
	ExpireAt   time.Time // 用来控制订单创建30分钟未支付自动取消，属于业务层面，出现在domain, usecase
	OrderItems []OrderItem
}

type OrderItem struct {
	ProductID        int64
	Quantity         int64
	SnapshotPrice    int64
	SnapshotCurrency string
	Price            int64 // 真正需要支付的价格，随币种变化
}

type Address struct {
	Street  string
	City    string
	State   string
	Country string
	Zipcode string
}

type Amount struct {
	Currency string
	Total    int64
}

type OrderStatus uint8

func (s OrderStatus) AsUint8() uint8 {
	return uint8(s)
}

const (
	OrderStatusUnknown = iota
	OrderStatusPending // 唯一的中间态
	OrderStatusCompleted
	OrderStatusCanceled
	// 后续再加等待发货，已发货什么的
)
