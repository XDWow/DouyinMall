package db

import "time"

// 订单采用主从表设计：
// orders 表存订单级信息，
// order_items 表存订单中商品明细，
// 以支持 1:N 关系、审计、对账与未来演进。

type OrderModel struct {
	ID       int64 `gorm:"primaryKey;autoIncrement"`
	UserID   int64 `gorm:"index:idx_userID_createdAt;index:idx_userID_status"`
	Phone    string
	Status   uint8 `gorm:"index:idx_status_expiredAt"`
	Currency string
	Total    int64

	// 地址
	Street  string
	City    string
	State   string `gorm:"index:idx_userID_status"`
	Country string
	ZipCode string

	CreatedAt time.Time	`gorm:"index:idx_userID_createdAt,sort:desc"`
	UpdatedAt time.Time
	ExpiredAt time.Time `gorm:"index:idx_status_expiredAt,sort:asc"`
	// ORM 关系声明，关系字段，不真正落库
	Items []OrderItemModel `gorm:"foreignKey:OrderID"`
}

func (OrderModel) TableName() string {
	return "orders"
}

type OrderItemModel struct {
	ID               int64 `gorm:"primaryKey;autoIncrement"`
	OrderID          int64
	ProductID        int64
	Quantity         int64
	SnapshotPrice    int64
	SnapshotCurrency string
	Price            int64
}

func (OrderItemModel) TableName() string {
	return "order_items"
}
