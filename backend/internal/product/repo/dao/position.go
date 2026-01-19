package dao

import (
	"context"

	"github.com/go-mysql-org/go-mysql/mysql"
	"gorm.io/gorm"
)

// key 用来区分不同实体表（product/merchant)
type CanalPosition struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Key        string `gorm:"type:varchar(255);not null;uniqueIndex;comment:位置标识"`
	BinlogFile string `gorm:"type:varchar(255);not null;comment:binlog文件名"`
	BinlogPos  uint32 `gorm:"type:int unsigned;not null;comment:binlog位置"`
	UpdatedAt  int64  `gorm:"type:bigint;not null;comment:更新时间"`
}

func (CanalPosition) TableName() string {
	return "canal_position"
}

type PositionDao interface {
	SavePosition(ctx context.Context, key string, pos mysql.Position) error
	LoadPosition(ctx context.Context, key string) (mysql.Position, error)
}

type gormPositionDao struct {
	db *gorm.DB
}

func NewGormPositionDao(db *gorm.DB) PositionDao {
	return &gormPositionDao{db: db}
}

func (d *gormPositionDao) SavePosition(ctx context.Context, key string, pos mysql.Position) error {
	position := CanalPosition{
		Key:        key,
		BinlogFile: pos.Name,
		BinlogPos:  pos.Pos,
	}

	return d.db.WithContext(ctx).
		Where("key = ?", key).
		Assign(map[string]interface{}{
			"binlog_file": pos.Name,
			"binlog_pos":  pos.Pos,
		}).
		FirstOrCreate(&position).Error
}

func (d *gormPositionDao) LoadPosition(ctx context.Context, key string) (mysql.Position, error) {
	var position CanalPosition
	err := d.db.WithContext(ctx).
		Where("key = ?", key).
		First(&position).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 没有保存的 position，返回空 position（从最新位置开始）
			return mysql.Position{}, nil
		}
		return mysql.Position{}, err
	}

	return mysql.Position{
		Name: position.BinlogFile,
		Pos:  position.BinlogPos,
	}, nil
}
