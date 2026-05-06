package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *GormRequestRepository) Create(ctx context.Context, request *domain.Request) error {
	model := db.SeckillRequest{
		RequestNo:  request.RequestNo,
		ActivityID: request.ActivityID,
		UserID:     request.UserID,
		Status:     request.Status,
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

func (r *GormRequestRepository) FindByActivityUser(ctx context.Context, activityID, userID int64) (*domain.Request, error) {
	var model db.SeckillRequest
	if err := r.db.WithContext(ctx).
		Where("activity_id = ? AND user_id = ?", activityID, userID).
		Order("id DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainRequest(model), nil
}

func (r *GormRequestRepository) AdvanceProcessing(ctx context.Context, evt domain.Event) (*domain.Request, error) {
	var next *domain.Request
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req db.SeckillRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ?", evt.RequestNo).
			First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRequestNotFound
			}
			return err
		}

		switch req.Status {
		case domain.RequestStatusOrderCreating, domain.RequestStatusSuccess, domain.RequestStatusFailed:
			next = toDomainRequest(req)
			return nil
		case domain.RequestStatusProcessing:
		default:
			return fmt.Errorf("seckill AdvanceProcessing: request_no=%s status=%s is not PROCESSING", evt.RequestNo, req.Status)
		}

		now := time.Now()
		res := tx.Model(&db.SeckillActivity{}).
			Where("id = ? AND available_stock >= ? AND status = ? AND start_time <= ? AND end_time >= ?",
				evt.ActivityID, 1, domain.ActivityStatusOnline, now, now).
			Updates(map[string]any{
				"available_stock": gorm.Expr("available_stock - 1"),
				"version":         gorm.Expr("version + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Model(&db.SeckillRequest{}).
				Where("request_no = ? AND status = ?", evt.RequestNo, domain.RequestStatusProcessing).
				Updates(map[string]any{
					"status":      domain.RequestStatusFailed,
					"fail_reason": domain.FailReasonOutOfStock,
				}).Error; err != nil {
				return err
			}
			req.Status = domain.RequestStatusFailed
			req.FailReason = domain.FailReasonOutOfStock
			next = toDomainRequest(req)
			return nil
		}

		qualification := db.SeckillQualification{
			ActivityID: evt.ActivityID,
			UserID:     evt.UserID,
			RequestNo:  evt.RequestNo,
		}
		if err := tx.Create(&qualification).Error; err != nil {
			if isDuplicate(err) {
				if err = tx.Model(&db.SeckillActivity{}).
					Where("id = ?", evt.ActivityID).
					Updates(map[string]any{
						"available_stock": gorm.Expr("available_stock + 1"),
						"version":         gorm.Expr("version + 1"),
					}).Error; err != nil {
					return err
				}
				if err = tx.Model(&db.SeckillRequest{}).
					Where("request_no = ? AND status = ?", evt.RequestNo, domain.RequestStatusProcessing).
					Updates(map[string]any{
						"status":      domain.RequestStatusFailed,
						"fail_reason": domain.FailReasonUserAlreadySucceeded,
					}).Error; err != nil {
					return err
				}
				req.Status = domain.RequestStatusFailed
				req.FailReason = domain.FailReasonUserAlreadySucceeded
				next = toDomainRequest(req)
				return nil
			}
			return err
		}

		res = tx.Model(&db.SeckillRequest{}).
			Where("request_no = ? AND status = ?", evt.RequestNo, domain.RequestStatusProcessing).
			Updates(map[string]any{
				"status":      domain.RequestStatusOrderCreating,
				"fail_reason": "",
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("seckill AdvanceProcessing: request_no=%s conditional update failed", evt.RequestNo)
		}

		req.Status = domain.RequestStatusOrderCreating
		req.FailReason = ""
		next = toDomainRequest(req)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

func (r *GormRequestRepository) CompleteOrderCreating(ctx context.Context, evt domain.Event) (*domain.Request, error) {
	var next *domain.Request
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req db.SeckillRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ?", evt.RequestNo).
			First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRequestNotFound
			}
			return err
		}

		switch req.Status {
		case domain.RequestStatusSuccess:
			next = toDomainRequest(req)
			return nil
		case domain.RequestStatusOrderCreating:
		default:
			return fmt.Errorf("seckill CompleteOrderCreating: request_no=%s status=%s is not ORDER_CREATING", evt.RequestNo, req.Status)
		}

		res := tx.Model(&db.SeckillRequest{}).
			Where("request_no = ? AND status = ?", evt.RequestNo, domain.RequestStatusOrderCreating).
			Updates(map[string]any{
				"status":      domain.RequestStatusSuccess,
				"fail_reason": "",
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("seckill CompleteOrderCreating: request_no=%s conditional update failed", evt.RequestNo)
		}

		req.Status = domain.RequestStatusSuccess
		req.FailReason = ""
		next = toDomainRequest(req)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

func (r *GormRequestRepository) RollbackOrderCreating(ctx context.Context, evt domain.Event, failReason string) (*domain.Request, error) {
	var next *domain.Request
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req db.SeckillRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ?", evt.RequestNo).
			First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRequestNotFound
			}
			return err
		}

		switch req.Status {
		case domain.RequestStatusFailed:
			next = toDomainRequest(req)
			return nil
		case domain.RequestStatusOrderCreating:
		default:
			return fmt.Errorf("seckill RollbackOrderCreating: request_no=%s status=%s is not ORDER_CREATING", evt.RequestNo, req.Status)
		}

		if err := tx.Model(&db.SeckillActivity{}).
			Where("id = ?", evt.ActivityID).
			Updates(map[string]any{
				"available_stock": gorm.Expr("available_stock + 1"),
				"version":         gorm.Expr("version + 1"),
			}).Error; err != nil {
			return err
		}

		if err := tx.Where("activity_id = ? AND user_id = ? AND request_no = ?", evt.ActivityID, evt.UserID, evt.RequestNo).
			Delete(&db.SeckillQualification{}).Error; err != nil {
			return err
		}

		res := tx.Model(&db.SeckillRequest{}).
			Where("request_no = ? AND status = ?", evt.RequestNo, domain.RequestStatusOrderCreating).
			Updates(map[string]any{
				"status":      domain.RequestStatusFailed,
				"fail_reason": failReason,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("seckill RollbackOrderCreating: request_no=%s conditional update failed", evt.RequestNo)
		}

		req.Status = domain.RequestStatusFailed
		req.FailReason = failReason
		next = toDomainRequest(req)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

func (r *GormRequestRepository) CloseByOrderResult(ctx context.Context, requestNo string, failReason string) (*domain.Request, bool, error) {
	var next *domain.Request
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req db.SeckillRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ?", requestNo).
			First(&req).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRequestNotFound
			}
			return err
		}

		switch req.Status {
		case domain.RequestStatusFailed:
			next = toDomainRequest(req)
			return nil
		case domain.RequestStatusProcessing:
		case domain.RequestStatusOrderCreating, domain.RequestStatusSuccess:
			if err := tx.Model(&db.SeckillActivity{}).
				Where("id = ?", req.ActivityID).
				Updates(map[string]any{
					"available_stock": gorm.Expr("available_stock + 1"),
					"version":         gorm.Expr("version + 1"),
				}).Error; err != nil {
				return err
			}
			if err := tx.Where("activity_id = ? AND user_id = ? AND request_no = ?", req.ActivityID, req.UserID, req.RequestNo).
				Delete(&db.SeckillQualification{}).Error; err != nil {
				return err
			}
		default:
			next = toDomainRequest(req)
			return nil
		}

		res := tx.Model(&db.SeckillRequest{}).
			Where("request_no = ? AND status <> ?", requestNo, domain.RequestStatusFailed).
			Updates(map[string]any{
				"status":      domain.RequestStatusFailed,
				"fail_reason": failReason,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			next = toDomainRequest(req)
			return nil
		}

		req.Status = domain.RequestStatusFailed
		req.FailReason = failReason
		next = toDomainRequest(req)
		changed = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return next, changed, nil
}

func (r *GormRequestRepository) MarkFail(ctx context.Context, requestNo string, failReason string) error {
	return r.db.WithContext(ctx).Model(&db.SeckillRequest{}).
		Where("request_no = ?", requestNo).
		Updates(map[string]any{
			"status":      domain.RequestStatusFailed,
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
	orderID := int64(0)
	if model.Status == domain.RequestStatusSuccess {
		if v, ok := domain.OrderIDFromRequestNo(model.RequestNo); ok {
			orderID = v
		}
	}
	return &domain.Request{
		ID:         model.ID,
		RequestNo:  model.RequestNo,
		ActivityID: model.ActivityID,
		UserID:     model.UserID,
		Status:     model.Status,
		OrderID:    orderID,
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
