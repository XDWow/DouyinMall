package dao

import (
	"context"

	"github.com/go-mysql-org/go-mysql/mysql"
	"gorm.io/gorm"
)

// key 鐢ㄦ潵鍖哄垎涓嶅悓瀹炰綋琛紙product/merchant)
type CanalPosition struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Key        string `gorm:"type:varchar(255);not null;uniqueIndex;comment:浣嶇疆鏍囪瘑"`
	BinlogFile string `gorm:"type:varchar(255);not null;comment:binlog鏂囦欢鍚?`
	BinlogPos  uint32 `gorm:"type:int unsigned;not null;comment:binlog浣嶇疆"`
	UpdatedAt  int64  `gorm:"type:bigint;not null;comment:鏇存柊鏃堕棿"`
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
			// 娌℃湁淇濆瓨鐨?position锛岃繑鍥炵┖ position锛堜粠鏈€鏂颁綅缃紑濮嬶級
			return mysql.Position{}, nil
		}
		return mysql.Position{}, err
	}

	return mysql.Position{
		Name: position.BinlogFile,
		Pos:  position.BinlogPos,
	}, nil
}


