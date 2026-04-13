package domain

import "context"

type ActivityRepository interface {
	Create(ctx context.Context, activity *Activity) error
	FindByID(ctx context.Context, activityID int64) (*Activity, error)
	UpdateStatus(ctx context.Context, activityID int64, status string) error
	DecreaseStock(ctx context.Context, activityID int64, requestNo string, quantity int32) error
	IncreaseStock(ctx context.Context, activityID int64, operationID string, quantity int32) error
	// TryDeductStockAndClaimSuccess 同一事务内：条件扣活动库存 + 插入 seckill_success（activity_id,user_id 唯一）。
	TryDeductStockAndClaimSuccess(ctx context.Context, activityID, userID int64, requestNo string, quantity int32) error
	// DeleteSuccessClaim 释放成功占有（创单失败等补偿路径）。
	DeleteSuccessClaim(ctx context.Context, activityID, userID int64) error
	UpdateSuccessOrderID(ctx context.Context, activityID, userID, orderID int64) error
}

type RequestRepository interface {
	Create(ctx context.Context, request *Request) error
	FindByRequestNo(ctx context.Context, requestNo string) (*Request, error)
	FindByActivityUser(ctx context.Context, activityID, userID int64) (*Request, error)
	// MarkQualified PROCESSING -> QUALIFIED（抢到资格、待支付）；已为 QUALIFIED 则幂等。
	MarkQualified(ctx context.Context, requestNo string) error
	MarkFail(ctx context.Context, requestNo string, failReason string) error
	// MarkFailByRequestNoIfActive 将 request_no 对应流水从 PROCESSING/QUALIFIED（及历史 SUCCESS）置为 FAIL，返回影响行数；0 表示已处理过取消/幂等。
	MarkFailByRequestNoIfActive(ctx context.Context, requestNo string, failReason string) (int64, error)
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
