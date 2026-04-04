package domain

import (
	"strings"
	"time"
)

type RequestType string
type RequestStatus string

const (
	RequestTypeReturn   RequestType = "return"
	RequestTypeExchange RequestType = "exchange"
)

const (
	StatusPendingReview RequestStatus = "pending_review"
	StatusApproved      RequestStatus = "approved"
	StatusRejected      RequestStatus = "rejected"
	StatusCanceled      RequestStatus = "canceled"
	StatusCompleted     RequestStatus = "completed"
)

type Request struct {
	ID          uint64
	RequestNo   string
	UserID      int64
	OrderID     int64
	ItemID      int64
	RequestType RequestType
	Reason      string
	Status      RequestStatus
	SessionID   string
	TraceID     string
	Metadata    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NormalizeRequestType(raw string) RequestType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RequestTypeExchange):
		return RequestTypeExchange
	default:
		return RequestTypeReturn
	}
}


