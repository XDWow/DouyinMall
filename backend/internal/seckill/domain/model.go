package domain

import "time"

const (
	ActivityStatusInit    = "INIT"
	ActivityStatusOnline  = "ONLINE"
	ActivityStatusOffline = "OFFLINE"
	ActivityStatusEnded   = "ENDED"
)

const (
	RequestStatusProcessing = "PROCESSING"
	// RequestStatusQualified 已抢到资格：创单成功，待支付（轮询到此可停，转支付/订单域）。
	RequestStatusQualified = "QUALIFIED"
	RequestStatusFail      = "FAIL"
	// RequestStatusLegacySuccess 历史库中可能仍为 SUCCESS，仅用于关单补偿等兼容。
	RequestStatusLegacySuccess = "SUCCESS"
)

const (
	FailReasonOutOfStock           = "OUT_OF_STOCK"
	FailReasonCreateOrderFail      = "CREATE_ORDER_FAIL"
	FailReasonDuplicate            = "DUPLICATE"
	FailReasonUserAlreadySucceeded = "USER_ALREADY_SUCCEEDED"
	FailReasonActivityMissing      = "ACTIVITY_NOT_FOUND"
	FailReasonActivityNotOpen      = "ACTIVITY_NOT_OPEN"
	FailReasonOrderCanceled        = "ORDER_CANCELED"
	FailReasonOrderRefunded        = "ORDER_REFUNDED"
)

type Activity struct {
	ID             int64
	ActivityName   string
	ProductID      int64
	SKUID          int64
	SeckillPrice   int64
	TotalStock     int32
	AvailableStock int32
	StartTime      time.Time
	EndTime        time.Time
	Status         string
	LimitPerUser   int32
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Request struct {
	ID         int64
	RequestNo  string
	ActivityID int64
	UserID     int64
	Quantity   int32
	Status     string
	OrderID    int64
	FailReason string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Result struct {
	RequestNo  string `json:"requestNo"`
	Status     string `json:"status"`
	OrderID    int64  `json:"orderId,omitempty"`
	FailReason string `json:"failReason,omitempty"`
}

type Event struct {
	RequestNo  string `json:"request_no"`
	ActivityID int64  `json:"activity_id"` // 秒杀活动 ID
	UserID     int64  `json:"user_id"`

	// 用于创建订单商品明细
	ProductID int64 `json:"product_id"`
	SKUID     int64 `json:"sku_id"`

	SeckillPrice int64 `json:"seckill_price"`
	Quantity     int32 `json:"quantity"`
}
