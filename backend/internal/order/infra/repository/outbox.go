package repository

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"gorm.io/gorm"
)

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) domain.OutboxRepository {
	return &outboxRepository{
		db: db,
	}
}

func (repo *outboxRepository) Add(ctx context.Context, eventType string, payload any) (int64, error) {
	conn := db.DBFromContext(ctx, repo.db)
	data, _ := json.Marshal(payload)
	model := db.OutboxEventModel{
		EventType:  eventType,
		Payload:    data,
		Status:     db.EventStatusPending,
		RetryCount: 0,
	}
	if err := conn.Create(&model).Error; err != nil {
		return 0, err
	}
	return model.ID, nil
}

func (repo *outboxRepository) BatchAdd(ctx context.Context, eventType string, payloads []any) ([]int64, error) {
	if len(payloads) == 0 {
		return nil, nil
	}

	conn := db.DBFromContext(ctx, repo.db)
	models := make([]db.OutboxEventModel, 0, len(payloads))
	for _, payload := range payloads {
		data, _ := json.Marshal(payload)
		models = append(models, db.OutboxEventModel{
			EventType:  eventType,
			Payload:    data,
			Status:     db.EventStatusPending,
			RetryCount: 0,
		})
	}

	// 使用 GORM CreateInBatches 批量插入。
	// 收益：减少网络 RTT、事务与连接开销。
	// 风险：单次 SQL 过大、锁时间过长，故单批控制在 100（可按压测调参）。
	if err := conn.CreateInBatches(models, 100).Error; err != nil {
		return nil, err
	}

	ids := make([]int64, len(models))
	for i := range models {
		ids[i] = models[i].ID
	}
	return ids, nil
}

func (repo *outboxRepository) ListPending(ctx context.Context, offset, limit int) ([]domain.OutboxEvent, error) {
	conn := db.DBFromContext(ctx, repo.db)
	models := make([]db.OutboxEventModel, 0, limit)
	res := conn.
		Where("status = ?", db.EventStatusPending).
		Offset(offset).
		Limit(limit).
		Find(&models)
	if res.Error != nil {
		return nil, res.Error
	}
	events := make([]domain.OutboxEvent, len(models))
	for i := range models {
		var evt domain.OrderStatusUpdateEvent
		_ = json.Unmarshal(models[i].Payload, &evt)
		events[i] = domain.OutboxEvent{
			ID:    models[i].ID,
			Event: evt,
		}
	}
	return events, nil
}

func (repo *outboxRepository) MarkSent(ctx context.Context, id int64) error {
	res := db.DBFromContext(ctx, repo.db).
		Where("id = ?", id).
		Update("status", db.EventStatusSent)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

func (repo *outboxRepository) BatchMarkSent(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	res := db.DBFromContext(ctx, repo.db).
		Model(&db.OutboxEventModel{}).
		Where("id IN ?", ids).
		Update("status", db.EventStatusSent)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (repo *outboxRepository) MarkFailed(ctx context.Context, id int64) error {
	res := db.DBFromContext(ctx, repo.db).
		Where("id = ?", id).
		Update("status", db.EventStatusFailed)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

func (repo *outboxRepository) IncreaseRetry(ctx context.Context, id int64) (int, error) {
	var retry int
	err := db.DBFromContext(ctx, repo.db).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&db.OutboxEventModel{}).
			Where("id = ?", id).
			Update("retry_count", gorm.Expr("retry_count + ?", 1))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrRecordNotFound
		}

		var model db.OutboxEventModel
		if err := tx.Select("retry_count").First(&model, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrRecordNotFound
			}
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


