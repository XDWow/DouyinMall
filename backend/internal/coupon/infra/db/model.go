package db

import "time"


type Coupon struct {
	ID int64 `gorm:"primaryKey;autoIncrement"`

	UserID  int64  `gorm:"not null;index:idx_user_id_status"`
	OrderID *int64 `gorm:"default:null;index:idx_order_id"` // 鍙负绌轰竴鑸敤鎸囬拡锛屾潵鍖哄垎绌?闆跺€硷紝绌鸿〃绀鸿繕娌＄敤鍛?

	TemplateID int64 `gorm:"not null;index:idx_user_template"`

	Status uint8 `gorm:"not null;default:1;index:idx_user_id_status"`

	ValidFrom time.Time  `gorm:"not null"`
	ValidTo   time.Time  `gorm:"not null;index:idx_valid_to"`
	UsedAt    *time.Time `gorm:"default:null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Template CouponTemplate `gorm:"foreignKey:TemplateID"`
}

func (Coupon) TableName() string {
	return "coupons"
}

type CouponTemplate struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(128);not null"`
	Description string `gorm:"type:varchar(512)"`

	CouponType int8 `gorm:"not null;default:1"`

	DiscountValue     int32  `gorm:"not null"`     
	MinOrderAmount    *int32 `gorm:"default:null"` 
	MaxDiscountAmount *int32 `gorm:"default:null"` 

	ApplicableProductIDs  string `gorm:"type:json"`
	ApplicableCategoryIDs string `gorm:"type:json"`

	ValidType      int8       `gorm:"not null;default:1"`
	ValidStartTime *time.Time `gorm:"default:null"`
	ValidEndTime   *time.Time `gorm:"default:null"`
	ValidDays      *int32     `gorm:"default:null"`

	TotalCount   int32 `gorm:"not null;default:0"`
	IssuedCount  int32 `gorm:"not null;default:0"`
	PerUserLimit int32 `gorm:"not null;default:1"`

	Status int8 `gorm:"not null;default:1"`

	MerchantID *int64 `gorm:"default:null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CouponTemplate) TableName() string {
	return "coupon_templates"
}

type CouponOperation struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	OperationID    string `gorm:"type:varchar(64);uniqueIndex:uk_operation_id;not null"`
	UserCouponID   int64  `gorm:"not null;index:idx_user_coupon_id"`
	OrderID        *int64 `gorm:"default:null;index:idx_order_id"`
	OperationType  string `gorm:"type:varchar(16);not null"`
	CreatedAt      time.Time
}

func (CouponOperation) TableName() string {
	return "coupon_operations"
}


