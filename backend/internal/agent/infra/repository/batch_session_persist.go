package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
	"gorm.io/gorm"
)

// BatchPersistSessionRounds 将多轮落库批次写入同一事务：先批量插入消息，再按批次顺序更新各 session 行。
func BatchPersistSessionRounds(ctx context.Context, db *gorm.DB, items []SessionRoundPersistBatchItem) error {
	if len(items) == 0 {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []agentdb.Message
		for i := range items {
			for j := range items[i].Messages {
				mw := items[i].Messages[j]
				if strings.TrimSpace(mw.Content) == "" {
					continue
				}
				if mw.CreatedAt.IsZero() {
					mw.CreatedAt = time.Now()
				}
				rows = append(rows, domainMessageToModel(mw))
			}
		}
		if len(rows) > 0 {
			if err := tx.CreateInBatches(&rows, 128).Error; err != nil {
				return err
			}
		}
		for i := range items {
			snap := items[i].Session
			if strings.TrimSpace(snap.SessionID) == "" {
				return fmt.Errorf("session_id is required")
			}
			now := time.Now()
			if err := tx.Model(&agentdb.Session{}).
				Where("session_id = ?", snap.SessionID).
				Updates(map[string]any{
					"status":       snap.Status,
					"last_message": snap.LastMessage,
					"total_turns":  snap.TotalTurns,
					"slots_json":   marshalSlots(snap.Slots),
					"updated_at":   now,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
