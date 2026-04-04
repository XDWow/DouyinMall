package db

import "time"

// 鏍规嵁 domain 鏉ヨ惤鍦帮紝璁捐鏁版嵁搴?
// 鍚庣画鏍规嵁 repo 鎿嶄綔锛屽啀鏉ヤ笉鏂姞绱㈠紩

type Coupon struct {
	ID int64 `gorm:"primaryKey;autoIncrement"`

	// 褰掑睘
	UserID  int64  `gorm:"not null;index:idx_user_id_status"`
	OrderID *int64 `gorm:"default:null;index:idx_order_id"` // 鍙负绌轰竴鑸敤鎸囬拡锛屾潵鍖哄垎绌?闆跺€硷紝绌鸿〃绀鸿繕娌＄敤鍛?

	// 澶栭敭锛屽彲鎵捐嚜宸辩殑灞炴€?
	TemplateID int64 `gorm:"not null;index:idx_user_template"`

	// 鐘舵€? 1-鏈娇鐢?2-宸查攣瀹?3-宸蹭娇鐢?4-宸查€€杩?
	Status uint8 `gorm:"not null;default:1;index:idx_user_id_status"`

	// 鏃堕棿鐩稿叧
	ValidFrom time.Time  `gorm:"not null"`
	ValidTo   time.Time  `gorm:"not null;index:idx_valid_to"`
	UsedAt    *time.Time `gorm:"default:null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// 杩欐槸涓€涓?ORM 灞傜殑鍏崇郴澹版槑锛屾寚瀵?GORM 濡備綍鍔犺浇鍜屾槧灏勬暟鎹細this.TemplateID  鈫? CouponTemplate.ID
	// 鐢ㄤ簬鎻忚堪妯″瀷涔嬮棿鐨勫叧鑱旓紝涓嶇瓑浠蜂簬鏁版嵁搴撳眰鐨勭害鏉?
	// 鏁版嵁搴撳彧淇濆瓨澶栭敭TemplateID锛孫RM 閫氳繃鍏宠仈澹版槑锛宲reload 鏃剁殑琛屼负锛氭牴鎹閿?ID 鍔犺浇骞舵槧灏勫叧鑱斿璞?
	Template CouponTemplate `gorm:"foreignKey:TemplateID"`
}

func (Coupon) TableName() string {
	return "coupons"
}

type CouponTemplate struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(128);not null"`
	Description string `gorm:"type:varchar(512)"`

	// 绫诲瀷: 1-婊″噺 2-鎶樻墸 3-鍥哄畾閲戦
	CouponType int8 `gorm:"not null;default:1"`

	// 浼樻儬瑙勫垯
	DiscountValue     int32  `gorm:"not null"`     // 鎶樻墸鍊?
	MinOrderAmount    *int32 `gorm:"default:null"` // 婊″噺闇€瑕佺殑锛屾渶浣庤鍗曢噾棰?
	MaxDiscountAmount *int32 `gorm:"default:null"` // 鎶樻墸闇€瑕佺殑锛屾渶澶ф姌鎵ｉ噾棰?

	// 閫傜敤鑼冨洿 (JSON)
	ApplicableProductIDs  string `gorm:"type:json"`
	ApplicableCategoryIDs string `gorm:"type:json"`

	// 鏈夋晥鏈熻缃?
	ValidType      int8       `gorm:"not null;default:1"` // 1-鍥哄畾鏃堕棿 2-棰嗗彇鍚嶯澶?
	ValidStartTime *time.Time `gorm:"default:null"`
	ValidEndTime   *time.Time `gorm:"default:null"`
	ValidDays      *int32     `gorm:"default:null"`

	// 鍙戞斁鎺у埗
	TotalCount   int32 `gorm:"not null;default:0"` // 0=鏃犻檺鍒?
	IssuedCount  int32 `gorm:"not null;default:0"`
	PerUserLimit int32 `gorm:"not null;default:1"`

	// 鐘舵€? 1-鍚敤 2-绂佺敤
	Status int8 `gorm:"not null;default:1"`

	// 鍟嗗锛屽鏋滄槸鍟嗗鍒革紝杩欎釜瀛楁涓嶄负绌?
	MerchantID *int64 `gorm:"default:null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CouponTemplate) TableName() string {
	return "coupon_templates"
}

// 浼樻儬鍒告搷浣滆褰曪紙骞傜瓑锛?
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


