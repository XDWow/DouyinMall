package repository

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/db"
	"gorm.io/gorm"
)

type paymentOutboxRepository struct {
	db *gorm.DB
}

func NewPaymentOutboxRepository(db *gorm.DB) domain.PaymentOutboxRepository {
	return &paymentOutboxRepository{db: db}
}

func (repo *paymentOutboxRepository) Add(ctx context.Context, eventType string, payload any) (int64, error) {
	data, _ := json.Marshal(payload)
	model := db.PaymentOutboxEventModel{
		EventType:  eventType,
		Payload:    data,
		Status:     db.EventStatusPending,
		RetryCount: 0,
	}
	if err := db.DBFromContext(ctx, repo.db).Create(&model).Error; err != nil {
		return 0, err
	}
	return model.ID, nil
}

func (repo *paymentOutboxRepository) ListPending(ctx context.Context, offset, limit int) ([]domain.PaymentOutboxEvent, error) {
	models := make([]db.PaymentOutboxEventModel, 0, limit)
	res := db.DBFromContext(ctx, repo.db).
		Where("status = ?", db.EventStatusPending).
		Offset(offset).
		Limit(limit).
		Find(&models)
	if res.Error != nil {
		return nil, res.Error
	}

	events := make([]domain.PaymentOutboxEvent, 0, len(models))
	for _, model := range models {
		var evt domain.PaymentStatusUpdateEvent
		if err := json.Unmarshal(model.Payload, &evt); err != nil {
			continue
		}
		events = append(events, domain.PaymentOutboxEvent{
			ID:    model.ID,
			Event: evt,
		})
	}
	return events, nil
}

func (repo *paymentOutboxRepository) MarkSent(ctx context.Context, id int64) error {
	res := db.DBFromContext(ctx, repo.db).
		Model(&db.PaymentOutboxEventModel{}).
		Where("id = ?", id).
		Update("status", db.EventStatusSent)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrPaymentNotFound
	}
	return nil
}

func (repo *paymentOutboxRepository) BatchMarkSent(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return db.DBFromContext(ctx, repo.db).
		Model(&db.PaymentOutboxEventModel{}).
		Where("id IN ?", ids).
		Update("status", db.EventStatusSent).Error
}

func (repo *paymentOutboxRepository) MarkFailed(ctx context.Context, id int64) error {
	res := db.DBFromContext(ctx, repo.db).
		Model(&db.PaymentOutboxEventModel{}).
		Where("id = ?", id).
		Update("status", db.EventStatusFailed)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrPaymentNotFound
	}
	return nil
}

func (repo *paymentOutboxRepository) IncreaseRetry(ctx context.Context, id int64) (int, error) {
	var retry int
	err := db.DBFromContext(ctx, repo.db).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&db.PaymentOutboxEventModel{}).
			Where("id = ?", id).
			Update("retry_count", gorm.Expr("retry_count + ?", 1))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrPaymentNotFound
		}

		var model db.PaymentOutboxEventModel
		if err := tx.Select("retry_count").First(&model, id).Error; err != nil {
			return err
		}
		retry = model.RetryCount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return retry, nil
}
