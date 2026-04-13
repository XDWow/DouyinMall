package db

import "time"

type SeckillActivity struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	ActivityName   string
	ProductID      int64 `gorm:"index:idx_product_sku"`
	SKUID          int64 `gorm:"index:idx_product_sku"`
	SeckillPrice   int64
	TotalStock     int32
	AvailableStock int32
	StartTime      time.Time `gorm:"index:idx_time_status"`
	EndTime        time.Time `gorm:"index:idx_time_status"`
	Status         string    `gorm:"type:varchar(32);index:idx_time_status"`
	LimitPerUser   int32
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (SeckillActivity) TableName() string { return "seckill_activity" }

type SeckillRequest struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	RequestNo  string `gorm:"type:varchar(64);uniqueIndex"`
	ActivityID int64  `gorm:"index:idx_seckill_request_activity_user"`
	UserID     int64  `gorm:"index:idx_seckill_request_activity_user"`
	Quantity   int32
	Status     string `gorm:"type:varchar(32);index"`
	OrderID    int64
	FailReason string `gorm:"type:varchar(64)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SeckillRequest) TableName() string { return "seckill_request" }

type SeckillOperation struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	OperationID string `gorm:"type:varchar(128);uniqueIndex"`
	ActivityID  int64
	RequestNo   string `gorm:"type:varchar(64);index"`
	OrderID     int64
	Delta       int32
	Type        string `gorm:"type:varchar(32)"`
	CreatedAt   time.Time
}

func (SeckillOperation) TableName() string { return "seckill_operation" }


