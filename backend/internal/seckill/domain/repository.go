package domain

import "context"

type ActivityRepository interface {
	Create(ctx context.Context, activity *Activity) error
	FindByID(ctx context.Context, activityID int64) (*Activity, error)
	UpdateStatus(ctx context.Context, activityID int64, status string) error
	DecreaseStock(ctx context.Context, activityID int64, requestNo string, quantity int32) error
	IncreaseStock(ctx context.Context, activityID int64, operationID string, quantity int32) error
}

type RequestRepository interface {
	Create(ctx context.Context, request *Request) error
	FindByRequestNo(ctx context.Context, requestNo string) (*Request, error)
	FindByOrderID(ctx context.Context, orderID int64) (*Request, error)
	FindByActivityUser(ctx context.Context, activityID, userID int64) (*Request, error)
	MarkSuccess(ctx context.Context, requestNo string, orderID int64) error
	MarkFail(ctx context.Context, requestNo string, failReason string) error
}

type Cache interface {
	SetActivity(ctx context.Context, activity *Activity) error
	GetActivity(ctx context.Context, activityID int64) (*Activity, error)
	SetActivityStock(ctx context.Context, activityID int64, stock int32) error
	AtomicReserve(ctx context.Context, activityID, userID int64, requestNo string, userTTLSeconds int64) (int64, error)
	Compensate(ctx context.Context, activityID, userID int64, quantity int32, removeUser bool) error
	IncreaseStock(ctx context.Context, activityID int64, quantity int32) error
	SetResult(ctx context.Context, result Result) error
	GetResult(ctx context.Context, requestNo string) (*Result, error)
}

type Producer interface {
	Publish(ctx context.Context, evt Event) error
}


