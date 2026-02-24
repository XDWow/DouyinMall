package db

import "time"

// 根据 domain 来落地，设计数据库
// 后续根据 repo 操作，再来不断加索引

type Coupon struct {
	ID int64 `gorm:"primaryKey;autoIncrement"`

	// 归属
	UserID  int64  `gorm:"not null;index:idx_user_id_status"`
	OrderID *int64 `gorm:"default:null;index:idx_order_id"` // 可为空一般用指针，来区分空/零值，空表示还没用呢

	// 外键，可找自己的属性
	TemplateID int64 `gorm:"not null;index:idx_user_template"`

	// 状态: 1-未使用 2-已锁定 3-已使用 4-已退还
	Status uint8 `gorm:"not null;default:1;index:idx_user_id_status"`

	// 时间相关
	ValidFrom time.Time  `gorm:"not null"`
	ValidTo   time.Time  `gorm:"not null;index:idx_valid_to"`
	UsedAt    *time.Time `gorm:"default:null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// 这是一个 ORM 层的关系声明，指导 GORM 如何加载和映射数据：this.TemplateID  →  CouponTemplate.ID
	// 用于描述模型之间的关联，不等价于数据库层的约束
	// 数据库只保存外键TemplateID，ORM 通过关联声明，preload 时的行为：根据外键 ID 加载并映射关联对象
	Template CouponTemplate `gorm:"foreignKey:TemplateID"`
}

func (Coupon) TableName() string {
	return "coupons"
}

type CouponTemplate struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(128);not null"`
	Description string `gorm:"type:varchar(512)"`

	// 类型: 1-满减 2-折扣 3-固定金额
	CouponType int8 `gorm:"not null;default:1"`

	// 优惠规则
	DiscountValue     int32  `gorm:"not null"`     // 折扣值
	MinOrderAmount    *int32 `gorm:"default:null"` // 满减需要的，最低订单金额
	MaxDiscountAmount *int32 `gorm:"default:null"` // 折扣需要的，最大折扣金额

	// 适用范围 (JSON)
	ApplicableProductIDs  string `gorm:"type:json"`
	ApplicableCategoryIDs string `gorm:"type:json"`

	// 有效期设置
	ValidType      int8       `gorm:"not null;default:1"` // 1-固定时间 2-领取后N天
	ValidStartTime *time.Time `gorm:"default:null"`
	ValidEndTime   *time.Time `gorm:"default:null"`
	ValidDays      *int32     `gorm:"default:null"`

	// 发放控制
	TotalCount   int32 `gorm:"not null;default:0"` // 0=无限制
	IssuedCount  int32 `gorm:"not null;default:0"`
	PerUserLimit int32 `gorm:"not null;default:1"`

	// 状态: 1-启用 2-禁用
	Status int8 `gorm:"not null;default:1"`

	// 商家，如果是商家券，这个字段不为空
	MerchantID *int64 `gorm:"default:null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CouponTemplate) TableName() string {
	return "coupon_templates"
}

// 优惠券操作记录（幂等）
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
