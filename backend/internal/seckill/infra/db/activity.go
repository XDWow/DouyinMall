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
	Status     string `gorm:"type:varchar(32);index"`
	FailReason string `gorm:"type:varchar(64)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SeckillRequest) TableName() string { return "seckill_request" }
