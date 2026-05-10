package dao

import (
	"context"

	"github.com/go-mysql-org/go-mysql/mysql"
	"gorm.io/gorm"
)

type CanalPosition struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Key        string `gorm:"type:varchar(255);not null;uniqueIndex;comment:Position key"`
	BinlogFile string `gorm:"type:varchar(255);not null;comment:Binlog file"`
	BinlogPos  uint32 `gorm:"type:int unsigned;not null;comment:Binlog position"`
	UpdatedAt  int64  `gorm:"type:bigint;not null;comment:Updated time"`
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
			return mysql.Position{}, nil
		}
		return mysql.Position{}, err
	}

	return mysql.Position{
		Name: position.BinlogFile,
		Pos:  position.BinlogPos,
	}, nil
}
