package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

type CreateActivityUseCase struct {
	activityRepo domain.ActivityRepository
	cache        domain.Cache
}

func NewCreateActivityUseCase(activityRepo domain.ActivityRepository, cache domain.Cache) *CreateActivityUseCase {
	return &CreateActivityUseCase{activityRepo: activityRepo, cache: cache}
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
	_ = uc.cache.SetActivity(ctx, activity)
	_ = uc.cache.SetActivityStock(ctx, activity.ID, activity.AvailableStock)
	return activity.ID, nil
}

type UpdateActivityStatusUseCase struct {
	activityRepo domain.ActivityRepository
	cache        domain.Cache
}

func NewUpdateActivityStatusUseCase(activityRepo domain.ActivityRepository, cache domain.Cache) *UpdateActivityStatusUseCase {
	return &UpdateActivityStatusUseCase{activityRepo: activityRepo, cache: cache}
}

func (uc *UpdateActivityStatusUseCase) Execute(ctx context.Context, activityID int64, status string) error {
	if err := uc.activityRepo.UpdateStatus(ctx, activityID, status); err != nil {
		return err
	}
	activity, err := uc.activityRepo.FindByID(ctx, activityID)
	if err != nil {
		return err
	}
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


