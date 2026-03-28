package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/db"
	"gorm.io/gorm"
)

type GormActivityRepository struct{ db *gorm.DB }
type GormRequestRepository struct{ db *gorm.DB }

func NewActivityRepository(dbConn *gorm.DB) domain.ActivityRepository {
	return &GormActivityRepository{db: dbConn}
}

func NewRequestRepository(dbConn *gorm.DB) domain.RequestRepository {
	return &GormRequestRepository{db: dbConn}
}

func (r *GormActivityRepository) Create(ctx context.Context, activity *domain.Activity) error {
	model := db.SeckillActivity{
		ActivityName:   activity.ActivityName,
		ProductID:      activity.ProductID,
		SKUID:          activity.SKUID,
		SeckillPrice:   activity.SeckillPrice,
		TotalStock:     activity.TotalStock,
		AvailableStock: activity.AvailableStock,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		Status:         activity.Status,
		LimitPerUser:   activity.LimitPerUser,
		Version:        activity.Version,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	activity.ID = model.ID
	return nil
}

func (r *GormActivityRepository) FindByID(ctx context.Context, activityID int64) (*domain.Activity, error) {
	var model db.SeckillActivity
	if err := r.db.WithContext(ctx).Where("id = ?", activityID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrActivityNotFound
		}
		return nil, err
	}
	return toDomainActivity(model), nil
}

func (r *GormActivityRepository) UpdateStatus(ctx context.Context, activityID int64, status string) error {
	res := r.db.WithContext(ctx).Model(&db.SeckillActivity{}).Where("id = ?", activityID).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrActivityNotFound
	}
	return nil
}

func (r *GormActivityRepository) DecreaseStock(ctx context.Context, activityID int64, requestNo string, quantity int32) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		op := db.SeckillOperation{
			OperationID: "deduct_" + requestNo,
			ActivityID:  activityID,
			RequestNo:   requestNo,
			Delta:       -quantity,
			Type:        "DEDUCT",
			CreatedAt:   time.Now(),
		}
		if err := tx.Create(&op).Error; err != nil {
			if isDuplicate(err) {
				return nil
			}
			return err
		}
		res := tx.Model(&db.SeckillActivity{}).
			Where("id = ? AND available_stock >= ? AND status = ? AND start_time <= ? AND end_time >= ?",
				activityID, quantity, domain.ActivityStatusOnline, time.Now(), time.Now()).
			Updates(map[string]interface{}{
				"available_stock": gorm.Expr("available_stock - ?", quantity),
				"version":         gorm.Expr("version + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrOutOfStock
		}
		return nil
	})
}

func (r *GormActivityRepository) IncreaseStock(ctx context.Context, activityID int64, operationID string, quantity int32) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		op := db.SeckillOperation{
			OperationID: operationID,
			ActivityID:  activityID,
			Delta:       quantity,
			Type:        "RESTORE",
			CreatedAt:   time.Now(),
		}
		if err := tx.Create(&op).Error; err != nil {
			if isDuplicate(err) {
				return nil
			}
			return err
		}
		return tx.Model(&db.SeckillActivity{}).
			Where("id = ?", activityID).
			Updates(map[string]interface{}{
				"available_stock": gorm.Expr("available_stock + ?", quantity),
				"version":         gorm.Expr("version + 1"),
			}).Error
	})
}

func (r *GormRequestRepository) Create(ctx context.Context, request *domain.Request) error {
	model := db.SeckillRequest{
		RequestNo:  request.RequestNo,
		ActivityID: request.ActivityID,
		UserID:     request.UserID,
		Quantity:   request.Quantity,
		Status:     request.Status,
		OrderID:    request.OrderID,
		FailReason: request.FailReason,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return domain.ErrDuplicateSeckill
		}
		return err
	}
	request.ID = model.ID
	return nil
}

func (r *GormRequestRepository) FindByRequestNo(ctx context.Context, requestNo string) (*domain.Request, error) {
	var model db.SeckillRequest
	if err := r.db.WithContext(ctx).Where("request_no = ?", requestNo).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRequestNotFound
		}
		return nil, err
	}
	return toDomainRequest(model), nil
}

func (r *GormRequestRepository) FindByOrderID(ctx context.Context, orderID int64) (*domain.Request, error) {
	var model db.SeckillRequest
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRequestNotFound
		}
		return nil, err
	}
	return toDomainRequest(model), nil
}

func (r *GormRequestRepository) FindByActivityUser(ctx context.Context, activityID, userID int64) (*domain.Request, error) {
	var model db.SeckillRequest
	if err := r.db.WithContext(ctx).Where("activity_id = ? AND user_id = ?", activityID, userID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainRequest(model), nil
}

func (r *GormRequestRepository) MarkSuccess(ctx context.Context, requestNo string, orderID int64) error {
	return r.db.WithContext(ctx).Model(&db.SeckillRequest{}).Where("request_no = ?", requestNo).Updates(map[string]interface{}{
		"status":   domain.RequestStatusSuccess,
		"order_id": orderID,
	}).Error
}

func (r *GormRequestRepository) MarkFail(ctx context.Context, requestNo string, failReason string) error {
	return r.db.WithContext(ctx).Model(&db.SeckillRequest{}).Where("request_no = ?", requestNo).Updates(map[string]interface{}{
		"status":      domain.RequestStatusFail,
		"fail_reason": failReason,
	}).Error
}

func toDomainActivity(model db.SeckillActivity) *domain.Activity {
	return &domain.Activity{
		ID:             model.ID,
		ActivityName:   model.ActivityName,
		ProductID:      model.ProductID,
		SKUID:          model.SKUID,
		SeckillPrice:   model.SeckillPrice,
		TotalStock:     model.TotalStock,
		AvailableStock: model.AvailableStock,
		StartTime:      model.StartTime,
		EndTime:        model.EndTime,
		Status:         model.Status,
		LimitPerUser:   model.LimitPerUser,
		Version:        model.Version,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func toDomainRequest(model db.SeckillRequest) *domain.Request {
	return &domain.Request{
		ID:         model.ID,
		RequestNo:  model.RequestNo,
		ActivityID: model.ActivityID,
		UserID:     model.UserID,
		Quantity:   model.Quantity,
		Status:     model.Status,
		OrderID:    model.OrderID,
		FailReason: model.FailReason,
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1062") || strings.Contains(msg, "Duplicate entry")
}
