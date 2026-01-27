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

func (repo *outboxRepository) Add(ctx context.Context, eventType string, payload any) error {
	data, _ := json.Marshal(payload)
	model := db.OutboxEventModel{
		EventType:  eventType,
		Payload:    data,
		Status:     db.EventStatusPending,
		RetryCount: 0,
	}
	return repo.db.WithContext(ctx).Create(&model).Error
}

func (repo *outboxRepository) BatchAdd(ctx context.Context, eventType string, payloads []any) error {
	if len(payloads) == 0 {
		return nil
	}

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

	// 调用 GORM 的批量插入接口
	// 批量插入的收益来自：减少 网络RTT / 事务 / 连接开销
	// 风险来自单次 SQL 太大、锁时间太长，所以控制批量大小 100（经验值，可以压测调参）
	return repo.db.WithContext(ctx).CreateInBatches(models, 100).Error
}

func (repo *outboxRepository) ListPending(ctx context.Context, offset, limit int) ([]domain.OutboxEvent, error) {
	models := make([]db.OutboxEventModel, 0, limit)
	res := repo.db.WithContext(ctx).
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
	res := repo.db.WithContext(ctx).
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
	
	res := repo.db.WithContext(ctx).
		Model(&db.OutboxEventModel{}).
		Where("id IN ?", ids).
		Update("status", db.EventStatusSent)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (repo *outboxRepository) MarkFailed(ctx context.Context, id int64) error {
	res := repo.db.WithContext(ctx).
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
	err := repo.db.WithContext(ctx).
		Where("id = ?", id).
		Update("retry_count", gorm.Expr("retry_count + ?", 1)). // 原子+1
		Scan(&retry).Error
	if err != nil {
		return 0, err
	}
	return retry, nil
}
