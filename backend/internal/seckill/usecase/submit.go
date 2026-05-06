package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
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
	log          logger.LoggerV1
}

func NewSubmitUseCase(activityRepo domain.ActivityRepository, requestRepo domain.RequestRepository, cache domain.Cache, producer domain.Producer, idGen IDGenerator, logs ...logger.LoggerV1) *SubmitUseCase {
	l := logger.NewNopLogger()
	if len(logs) > 0 && logs[0] != nil {
		l = logs[0]
	}
	return &SubmitUseCase{activityRepo: activityRepo, requestRepo: requestRepo, cache: cache, producer: producer, idGen: idGen, log: l}
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
		return &domain.Result{Status: domain.RequestStatusFailed, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityOffline
	case now.Before(activity.StartTime):
		return &domain.Result{Status: domain.RequestStatusFailed, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityNotStarted
	case now.After(activity.EndTime):
		return &domain.Result{Status: domain.RequestStatusFailed, FailReason: domain.FailReasonActivityNotOpen}, domain.ErrActivityEnded
	}

	requestNo := uc.idGen.GenerateID()
	result := &domain.Result{
		RequestNo: requestNo,
		Status:    domain.RequestStatusProcessing,
	}
	evt := domain.Event{
		RequestNo:    requestNo,
		ActivityID:   activity.ID,
		UserID:       cmd.UserID,
		ProductID:    activity.ProductID,
		SKUID:        activity.SKUID,
		SeckillPrice: activity.SeckillPrice,
	}

	tx, err := uc.producer.Prepare(ctx, evt)
	if err != nil {
		uc.log.Error("prepare seckill transaction message failed",
			logger.Error(err),
			logger.String("requestNo", requestNo),
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID))
		return nil, err
	}

	code, err := uc.cache.AtomicReserve(ctx, cmd.ActivityID, cmd.UserID, requestNo, userMarkerTTL(activity.EndTime))
	if err != nil {
		// Leave the half message unresolved and let the broker check Redis later.
		uc.log.Error("reserve seckill redis stock failed, wait broker transaction check",
			logger.Error(err),
			logger.String("requestNo", requestNo),
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID))
		return result, nil
	}

	switch code {
	case 1:
		_ = tx.Rollback()
		return &domain.Result{Status: domain.RequestStatusFailed, FailReason: domain.FailReasonOutOfStock}, domain.ErrOutOfStock
	case 2:
		_ = tx.Rollback()
		return &domain.Result{Status: domain.RequestStatusFailed, FailReason: domain.FailReasonDuplicate}, domain.ErrDuplicateSeckill
	}

	if err = tx.Commit(); err != nil {
		// The broker can still recover by checking Redis state later.
		uc.log.Error("commit seckill transaction message failed, wait broker transaction check",
			logger.Error(err),
			logger.String("requestNo", requestNo),
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID))
		return result, nil
	}
	uc.log.Info("seckill transaction message committed",
		logger.String("requestNo", requestNo),
		logger.Int64("activityID", activity.ID),
		logger.Int64("userID", cmd.UserID))
	return result, nil
}
