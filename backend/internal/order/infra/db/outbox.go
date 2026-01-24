package db

import "time"

type OutboxEventModel struct {
	ID         int64       `gorm:"primaryKey;autoIncrement"`
	EventType  string      `gorm:"size:64;index"`
	Payload    []byte      `gorm:"type:json"`
	Status     EventStatus `gorm:"size:16;index"` // PENDING / SENT / FAILED
	RetryCount int
	CreatedAt  time.Time
}

type EventStatus uint8

const (
	EventStatusUnknow = iota
	EventStatusPending
	EventStatusSent
	EventStatusFailed
)
