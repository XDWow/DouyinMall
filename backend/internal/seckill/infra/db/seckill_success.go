package db

import "time"

// SeckillSuccess 秒杀成功占有：与尝试流水 seckill_request 分离；唯一 (activity_id,user_id) 表达一人一单。
type SeckillSuccess struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	ActivityID int64  `gorm:"uniqueIndex:uk_seckill_success_activity_user"`
	UserID     int64  `gorm:"uniqueIndex:uk_seckill_success_activity_user"`
	RequestNo  string `gorm:"type:varchar(64);index"`
	OrderID    int64  `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SeckillSuccess) TableName() string { return "seckill_success" }
