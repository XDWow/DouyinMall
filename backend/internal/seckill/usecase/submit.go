package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

type IDGenerator interface {
	GenerateID() string
}

type SubmitUseCase struct {
	activityRepo domain.ActivityRepository
	requestRepo  domain.RequestRepository
	cache        domain.Cache
	producer     domain.Producer
	idGen        IDGenerator
}

func NewSubmitUseCase(activityRepo domain.ActivityRepository, requestRepo domain.RequestRepository, cache domain.Cache, producer domain.Producer, idGen IDGenerator) *SubmitUseCase {
	return &SubmitUseCase{activityRepo: activityRepo, requestRepo: requestRepo, cache: cache, producer: producer, idGen: idGen}
}

type SubmitCmd struct {
	ActivityID int64
	UserID     int64
}

func userMarkerTTL(endTime time.Time) int64 {
	ttl := time.Until(endTime) + 24*time.Hour
	if ttl < resultTTLForRequest() {
		ttl = resultTTLForRequest()
	}
	return int64(ttl.Seconds())
}

func resultTTLForRequest() time.Duration {
	return 24 * time.Hour
}

func (uc *SubmitUseCase) Execute(ctx context.Context, cmd SubmitCmd) (*domain.Result, error) {
	activity, err := uc.cache.GetActivity(ctx, cmd.ActivityID)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		activity, err = uc.activityRepo.FindByID(ctx, cmd.ActivityID)
		if err != nil {
			return nil, err
		}
		_ = uc.cache.SetActivity(ctx, activity)
	}

	now := time.Now()
	switch {
	case activity.Status != domain.ActivityStatusOnline:
		return &domain.Result{Status: domain.RequestStatusFail, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityOffline
	case now.Before(activity.StartTime):
		return &domain.Result{Status: domain.RequestStatusFail, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityNotStarted
	case now.After(activity.EndTime):
		return &domain.Result{Status: domain.RequestStatusFail, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityEnded
	}

	requestNo := uc.idGen.GenerateID()
	code, err := uc.cache.AtomicReserve(ctx, cmd.ActivityID, cmd.UserID, requestNo, userMarkerTTL(activity.EndTime))
	if err != nil {
		return nil, err
	}
	switch code {
	case 1: // 棰勬墸搴撳瓨鏄惁鎴愬姛锛?
		return &domain.Result{Status: domain.RequestStatusFail, FailReason: domain.FailReasonOutOfStock}, domain.ErrOutOfStock
	case 2: // 涓€浜轰竴鍗?
		return &domain.Result{Status: domain.RequestStatusFail, FailReason: domain.FailReasonDuplicate}, domain.ErrDuplicateSeckill
	}
	// 閫氳繃 redis 鎷︽埅鏍￠獙锛屽ぇ閮ㄥ垎娴侀噺浼氭嫤鍦ㄨ繖
	result := domain.Result{RequestNo: requestNo, Status: domain.RequestStatusProcessing}

	evt := domain.Event{
		RequestNo:    requestNo,
		ActivityID:   activity.ID,
		UserID:       cmd.UserID,
		ProductID:    activity.ProductID,
		SKUID:        activity.SKUID,
		SeckillPrice: activity.SeckillPrice,
		Quantity:     1,
	}
	if err = uc.producer.Publish(ctx, evt); err != nil {
		_ = uc.cache.Compensate(ctx, activity.ID, cmd.UserID, 1, true)
		_ = uc.cache.SetResult(ctx, domain.Result{RequestNo: requestNo, Status: domain.RequestStatusFail, FailReason: "PUBLISH_FAIL"})
		return nil, fmt.Errorf("publish seckill event failed: %w", err)
	}

	return &result, nil
}


