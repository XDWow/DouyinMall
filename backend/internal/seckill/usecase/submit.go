package usecase

import (
	"context"
	"os"
	"strings"
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
	soldOut      domain.SoldOutMarker
	producer     domain.Producer
	idGen        IDGenerator
	log          logger.LoggerV1
}

var submitStageTimingEnabled = isSubmitStageTimingEnabled()

func NewSubmitUseCase(activityRepo domain.ActivityRepository, requestRepo domain.RequestRepository, cache domain.Cache, soldOut domain.SoldOutMarker, producer domain.Producer, idGen IDGenerator, logs ...logger.LoggerV1) *SubmitUseCase {
	l := logger.NewNopLogger()
	if len(logs) > 0 && logs[0] != nil {
		l = logs[0]
	}
	if soldOut == nil {
		soldOut = domain.NewNopSoldOutMarker()
	}
	return &SubmitUseCase{
		activityRepo: activityRepo,
		requestRepo:  requestRepo,
		cache:        cache,
		soldOut:      soldOut,
		producer:     producer,
		idGen:        idGen,
		log:          l,
	}
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
	startedAt := time.Now()

	activityLookupStartedAt := time.Now()
	activity, err := uc.cache.GetActivity(ctx, cmd.ActivityID)
	activityLookupElapsed := time.Since(activityLookupStartedAt)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		dbLookupStartedAt := time.Now()
		activity, err = uc.activityRepo.FindByID(ctx, cmd.ActivityID)
		if err != nil {
			return nil, err
		}
		activityLookupElapsed += time.Since(dbLookupStartedAt)
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

	// Once the current node has confirmed the activity is sold out, tail traffic
	// can fail fast locally instead of continuing to hit Redis.
	if uc.soldOut.IsSoldOut(activity.ID) {
		uc.log.Info("local sold-out marker hit, skip redis reserve",
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID))
		return &domain.Result{
			Status:     domain.RequestStatusFailed,
			FailReason: domain.FailReasonOutOfStock,
		}, domain.ErrOutOfStock
	}

	requestNo := uc.idGen.GenerateID()
	evt := domain.Event{
		RequestNo:    requestNo,
		ActivityID:   activity.ID,
		UserID:       cmd.UserID,
		ProductID:    activity.ProductID,
		SKUID:        activity.SKUID,
		SeckillPrice: activity.SeckillPrice,
	}

	result, err := uc.producer.Submit(ctx, evt, userMarkerTTL(activity.EndTime))
	producerElapsed := time.Since(startedAt) - activityLookupElapsed
	totalElapsed := time.Since(startedAt)
	if submitStageTimingEnabled && (totalElapsed > 200*time.Millisecond || activityLookupElapsed > 20*time.Millisecond || producerElapsed > 100*time.Millisecond) {
		uc.log.Warn("slow seckill submit stages",
			logger.String("requestNo", requestNo),
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID),
			logger.Int64("activity_lookup_ms", activityLookupElapsed.Milliseconds()),
			logger.Int64("producer_submit_ms", producerElapsed.Milliseconds()),
			logger.Int64("submit_total_ms", totalElapsed.Milliseconds()))
	}
	if result != nil && result.Status == domain.RequestStatusProcessing && result.RequestNo != "" {
		uc.persistProcessingResult(result.RequestNo)
	}
	if err != nil {
		if result != nil {
			return result, err
		}
		uc.log.Error("send seckill transaction message failed",
			logger.Error(err),
			logger.String("requestNo", requestNo),
			logger.Int64("activityID", activity.ID),
			logger.Int64("userID", cmd.UserID))
		return nil, err
	}
	return result, nil
}

func (uc *SubmitUseCase) persistProcessingResult(requestNo string) {
	go func() {
		_ = uc.cache.SetResult(context.Background(), domain.Result{
			RequestNo: requestNo,
			Status:    domain.RequestStatusProcessing,
		})
	}()
}

func isSubmitStageTimingEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SECKILL_STAGE_TIMING"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
