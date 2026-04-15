package db

import "time"

// 订单采用主从表设计：
// orders 存订单主信息；
// order_items 存订单行商品明细；
// 支持 1:N、审计、对账与后续演进。

type OrderModel struct {
	ID            int64 `gorm:"primaryKey;autoIncrement"`
	UserID        int64 `gorm:"index:idx_userID_createdAt;index:idx_userID_status"`
	Phone         string
	Remark        string `gorm:"type:varchar(512)"`
	Status        uint8  `gorm:"index:idx_status_expiredAt"`
	OrderKind     string `gorm:"column:order_kind;type:varchar(32);index"`
	ActivityID    int64  `gorm:"index"`
	Currency      string
	Total         int64
	PayableTotal  int64
	DiscountTotal int64

	// 地址
	Street  string
	City    string
	State   string `gorm:"index:idx_userID_status"`
	Country string
	ZipCode string

	CreatedAt time.Time `gorm:"index:idx_userID_createdAt,sort:desc"`
	UpdatedAt time.Time
	ExpiredAt time.Time `gorm:"index:idx_status_expiredAt,sort:asc"`
	// GORM：与 OrderItem 的一对多关联
	Items []OrderItemModel `gorm:"foreignKey:OrderID"`
}

func (OrderModel) TableName() string {
	return "orders"
}

type OrderItemModel struct {
	ID               int64 `gorm:"primaryKey;autoIncrement"`
	OrderID          int64
	ProductID        int64
	SKUID            int64
	Quantity         int64
	SnapshotPrice    int64
	SnapshotCurrency string
	Price            int64
}

func (OrderItemModel) TableName() string {
	return "order_items"
}
