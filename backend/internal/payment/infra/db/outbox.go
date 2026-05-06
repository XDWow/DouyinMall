package db

import "time"

type PaymentOutboxEventModel struct {
	ID         int64       `gorm:"primaryKey;autoIncrement"`
	EventType  string      `gorm:"size:64;index"`
	Payload    []byte      `gorm:"type:json"`
	Status     EventStatus `gorm:"size:16;index"`
	RetryCount int
	CreatedAt  time.Time
}

func (PaymentOutboxEventModel) TableName() string {
	return "payment_outbox_events"
}

type EventStatus uint8

const (
	EventStatusUnknown EventStatus = iota
	EventStatusPending
	EventStatusSent
	EventStatusFailed
)
