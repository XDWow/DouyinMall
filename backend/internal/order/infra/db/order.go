package db

import "time"

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

	Items []OrderItemModel `gorm:"foreignKey:OrderID"`
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
