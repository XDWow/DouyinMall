package domain

import "context"

type ActivityRepository interface {
	Create(ctx context.Context, activity *Activity) error
	FindByID(ctx context.Context, activityID int64) (*Activity, error)
	UpdateStatus(ctx context.Context, activityID int64, status string) error
}

type RequestRepository interface {
	Create(ctx context.Context, request *Request) error
	FindByRequestNo(ctx context.Context, requestNo string) (*Request, error)
	FindByActivityUser(ctx context.Context, activityID, userID int64) (*Request, error)
	AdvanceProcessing(ctx context.Context, evt Event) (*Request, error)
	CompleteOrderCreating(ctx context.Context, evt Event) (*Request, error)
	RollbackOrderCreating(ctx context.Context, evt Event, failReason string) (*Request, error)
	CloseByOrderResult(ctx context.Context, requestNo string, failReason string) (*Request, bool, error)
	MarkFail(ctx context.Context, requestNo string, failReason string) error
}

type Cache interface {
	SetActivity(ctx context.Context, activity *Activity) error
	GetActivity(ctx context.Context, activityID int64) (*Activity, error)
	SetActivityStock(ctx context.Context, activityID int64, stock int32) error
	AtomicReserve(ctx context.Context, activityID, userID int64, requestNo string, userTTLSeconds int64) (int64, error)
	Compensate(ctx context.Context, activityID, userID int64, requestNo string, result Result) error
	SetResult(ctx context.Context, result Result) error
	GetResult(ctx context.Context, requestNo string) (*Result, error)
	ResolveTransaction(ctx context.Context, activityID, userID int64, requestNo string) (TransactionResolution, error)
}

type Producer interface {
	Submit(ctx context.Context, evt Event, userTTLSeconds int64) (*Result, error)
}
