package fixer

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/pkg/migrator"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OverrideFixer[T migrator.Entity] struct {
	base   *gorm.DB
	target *gorm.DB
	//columns []string
}

func NewOverrideFixer[T migrator.Entity](base *gorm.DB, target *gorm.DB) (*OverrideFixer[T], error) {
	// 鍦ㄨ繖閲岄渶瑕佹煡璇竴涓嬫暟鎹簱涓┒绔熸湁鍝簺鍒?
	//var t T
	//rows, err := base.Model(&t).Limit(1).Rows()
	//if err != nil {
	//	return nil, err
	//}
	//columns, err := rows.Columns()
	//if err != nil {
	//	return nil, err
	//}
	return &OverrideFixer[T]{
		base:   base,
		target: target,
		//columns: columns,
	}, nil
}

// 鎴戞嬁鍒版湁闂鐨?id 锛屽啀鍒ゆ柇鏄粈涔堥棶棰?
func (o *OverrideFixer[T]) Fix(ctx context.Context, id int64) error {
	var src T
	err := o.base.WithContext(ctx).Where("id = ?", id).First(&src).Error
	// 涓夌鎯呭喌锛岄€氳繃鏌?src 杩斿洖鐨?err + upsert 灏辫兘鍒嗗埆澶勭悊
	switch err {
	case nil:
		return o.target.Clauses(&clause.OnConflict{
			//DoUpdates: clause.AssignmentColumns(o.columns),
			UpdateAll: true,
		}).Create(&src).Error
	case gorm.ErrRecordNotFound:
		return o.target.Delete("id = ?", id).Error
	default:
		return err
	}
}


