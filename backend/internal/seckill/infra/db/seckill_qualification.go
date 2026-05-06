package db

import "time"

// SeckillQualification expresses one-user-one-order at the DB layer.
type SeckillQualification struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	ActivityID int64  `gorm:"uniqueIndex:uk_seckill_qualification_activity_user"`
	UserID     int64  `gorm:"uniqueIndex:uk_seckill_qualification_activity_user"`
	RequestNo  string `gorm:"type:varchar(64);index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SeckillQualification) TableName() string { return "seckill_qualification" }
