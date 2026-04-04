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
	RequestStatusSuccess    = "SUCCESS"
	RequestStatusFail       = "FAIL"
)

const (
	FailReasonOutOfStock      = "OUT_OF_STOCK"
	FailReasonCreateOrderFail = "CREATE_ORDER_FAIL"
	FailReasonDuplicate       = "DUPLICATE"
	FailReasonActivityMissing = "ACTIVITY_NOT_FOUND"
	FailReasonActivityNotOpen = "ACTIVITY_NOT_OPEN"
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
	ActivityID int64  `json:"activity_id"` // 绠＄鏉€娲诲姩
	UserID     int64  `json:"user_id"`

	// 鐢ㄦ潵寤虹珛璁㈠崟鍟嗗搧鏄庣粏
	ProductID int64 `json:"product_id"`
	SKUID     int64 `json:"sku_id"`

	SeckillPrice int64 `json:"seckill_price"`
	Quantity     int32 `json:"quantity"`
}


