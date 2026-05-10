package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

type CreateActivityUseCase struct {
	activityRepo domain.ActivityRepository
	cache        domain.Cache
	soldOut      domain.SoldOutMarker
}

func NewCreateActivityUseCase(activityRepo domain.ActivityRepository, cache domain.Cache, soldOut domain.SoldOutMarker) *CreateActivityUseCase {
	if soldOut == nil {
		soldOut = domain.NewNopSoldOutMarker()
	}
	return &CreateActivityUseCase{activityRepo: activityRepo, cache: cache, soldOut: soldOut}
}

type CreateActivityCmd struct {
	ActivityName string
	ProductID    int64
	SKUID        int64
	SeckillPrice int64
	TotalStock   int32
	StartTime    time.Time
	EndTime      time.Time
	Status       string
	LimitPerUser int32
}

func (uc *CreateActivityUseCase) Execute(ctx context.Context, cmd CreateActivityCmd) (int64, error) {
	activity := &domain.Activity{
		ActivityName:   cmd.ActivityName,
		ProductID:      cmd.ProductID,
		SKUID:          cmd.SKUID,
		SeckillPrice:   cmd.SeckillPrice,
		TotalStock:     cmd.TotalStock,
		AvailableStock: cmd.TotalStock,
		StartTime:      cmd.StartTime,
		EndTime:        cmd.EndTime,
		Status:         cmd.Status,
		LimitPerUser:   cmd.LimitPerUser,
	}
	if activity.Status == "" {
		activity.Status = domain.ActivityStatusInit
	}
	if activity.LimitPerUser == 0 {
		activity.LimitPerUser = 1
	}
	if err := uc.activityRepo.Create(ctx, activity); err != nil {
		return 0, err
	}
	// 新活动创建后先清一次本机售罄标记，避免复用旧状态。
	uc.soldOut.Clear(activity.ID)
	_ = uc.cache.SetActivity(ctx, activity)
	_ = uc.cache.SetActivityStock(ctx, activity.ID, activity.AvailableStock)
	return activity.ID, nil
}

type UpdateActivityStatusUseCase struct {
	activityRepo domain.ActivityRepository
	cache        domain.Cache
	soldOut      domain.SoldOutMarker
}

func NewUpdateActivityStatusUseCase(activityRepo domain.ActivityRepository, cache domain.Cache, soldOut domain.SoldOutMarker) *UpdateActivityStatusUseCase {
	if soldOut == nil {
		soldOut = domain.NewNopSoldOutMarker()
	}
	return &UpdateActivityStatusUseCase{activityRepo: activityRepo, cache: cache, soldOut: soldOut}
}

func (uc *UpdateActivityStatusUseCase) Execute(ctx context.Context, activityID int64, status string) error {
	if err := uc.activityRepo.UpdateStatus(ctx, activityID, status); err != nil {
		return err
	}
	activity, err := uc.activityRepo.FindByID(ctx, activityID)
	if err != nil {
		return err
	}
	// 活动状态被人工调整后，保守起见清掉本机售罄标记。
	uc.soldOut.Clear(activityID)
	return uc.cache.SetActivity(ctx, activity)
}

type GetActivityUseCase struct {
	activityRepo domain.ActivityRepository
	cache        domain.Cache
}

func NewGetActivityUseCase(activityRepo domain.ActivityRepository, cache domain.Cache) *GetActivityUseCase {
	return &GetActivityUseCase{activityRepo: activityRepo, cache: cache}
}

func (uc *GetActivityUseCase) Execute(ctx context.Context, activityID int64) (*domain.Activity, error) {
	activity, err := uc.cache.GetActivity(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if activity != nil {
		return activity, nil
	}
	activity, err = uc.activityRepo.FindByID(ctx, activityID)
	if err != nil {
		return nil, err
	}
	_ = uc.cache.SetActivity(ctx, activity)
	return activity, nil
}
