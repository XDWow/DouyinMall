package fixer

import (
	"context"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OverrideFixer[T migrator.Entity] struct {
	base   *gorm.DB
	target *gorm.DB
	//columns []string
}

func NewOverrideFixer[T migrator.Entity](base *gorm.DB, target *gorm.DB) (*OverrideFixer[T], error) {
	// 在这里需要查询一下数据库中究竟有哪些列
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

// 我拿到有问题的 id ，再判断是什么问题
func (o *OverrideFixer[T]) Fix(ctx context.Context, id int64) error {
	var src T
	err := o.base.WithContext(ctx).Where("id = ?", id).First(&src).Error
	// 三种情况，通过查 src 返回的 err + upsert 就能分别处理
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
