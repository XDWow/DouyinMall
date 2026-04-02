package db

import "time"

type AfterSaleRequest struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	RequestNo   string    `gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID      int64     `gorm:"index;not null"`
	OrderID     int64     `gorm:"index;not null"`
	ItemID      int64     `gorm:"index;default:0"`
	RequestType string    `gorm:"type:varchar(16);not null"`
	Reason      string    `gorm:"type:varchar(255);not null"`
	Status      string    `gorm:"type:varchar(32);not null"`
	SessionID   string    `gorm:"type:varchar(64)"`
	TraceID     string    `gorm:"type:varchar(64)"`
	Metadata    string    `gorm:"type:longtext"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (AfterSaleRequest) TableName() string { return "after_sale_requests" }
