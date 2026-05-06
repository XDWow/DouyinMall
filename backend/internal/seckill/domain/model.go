package domain

import (
	"strconv"
	"time"
)

const (
	ActivityStatusInit    = "INIT"
	ActivityStatusOnline  = "ONLINE"
	ActivityStatusOffline = "OFFLINE"
	ActivityStatusEnded   = "ENDED"
)

const (
	RequestStatusProcessing    = "PROCESSING"
	RequestStatusOrderCreating = "ORDER_CREATING"
	RequestStatusSuccess       = "SUCCESS"
	RequestStatusFailed        = "FAILED"
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

type TransactionResolution int

const (
	TransactionResolutionUnknown TransactionResolution = iota
	TransactionResolutionCommit
	TransactionResolutionRollback
)

type Event struct {
	RequestNo  string `json:"request_no"`
	ActivityID int64  `json:"activity_id"`
	UserID     int64  `json:"user_id"`
	ProductID  int64  `json:"product_id"`
	SKUID      int64  `json:"sku_id"`

	SeckillPrice int64 `json:"seckill_price"`
}

type DeadLetterEvent struct {
	Event           Event  `json:"event"`
	Reason          string `json:"reason"`
	SourceMessageID string `json:"source_message_id,omitempty"`
	DeliveryAttempt int32  `json:"delivery_attempt,omitempty"`
}

func OrderIDFromRequestNo(requestNo string) (int64, bool) {
	orderID, err := strconv.ParseInt(requestNo, 10, 64)
	if err != nil {
		return 0, false
	}
	return orderID, true
}
