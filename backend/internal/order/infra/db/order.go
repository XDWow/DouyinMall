package db

import "time"

// 璁㈠崟閲囩敤涓讳粠琛ㄨ璁★細
// orders 琛ㄥ瓨璁㈠崟绾т俊鎭紝
// order_items 琛ㄥ瓨璁㈠崟涓晢鍝佹槑缁嗭紝
// 浠ユ敮鎸?1:N 鍏崇郴銆佸璁°€佸璐︿笌鏈潵婕旇繘銆?

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

	// 鍦板潃
	Street  string
	City    string
	State   string `gorm:"index:idx_userID_status"`
	Country string
	ZipCode string

	CreatedAt time.Time `gorm:"index:idx_userID_createdAt,sort:desc"`
	UpdatedAt time.Time
	ExpiredAt time.Time `gorm:"index:idx_status_expiredAt,sort:asc"`
	// ORM 鍏崇郴澹版槑锛屽叧绯诲瓧娈碉紝涓嶇湡姝ｈ惤搴?
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


