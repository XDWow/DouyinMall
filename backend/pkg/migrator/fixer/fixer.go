package fixer

import (
	"context"
	"errors"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator/events"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Fixer[T migrator.Entity] struct {
	base    *gorm.DB
	target  *gorm.DB
	columns []string
}

// 最一了百了的写法
// 不管三七二十一，我TM直接覆盖
// 把 event 当成一个触发器，不依赖的 event 的具体内容（ID 必须不可变）
// 修复这里，也改成批量？？
func (f *Fixer[T]) Fix(ctx context.Context, evt events.InconsistentEvent) error {
	panic("实现我")
}

// 更简洁，并且upsert语句，解决了并发问题
// 校验的时候TargetMissing，如果到修复时已经有了，再插入肯定失败，但upsert能成功
func (f *Fixer[T]) FixV1(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ,
		events.InconsistentEventTypeTargetMissing:
		// 更新 target
		// 去 base 里面查出来，可能有几种情况，都会变的
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("超时或主动取消")
		case gorm.ErrRecordNotFound:
			// 变了，base 删除了，target 也要删除
			return f.target.WithContext(ctx).
				Where("id = ?", evt.ID).Delete(&t).Error
		case nil:
			return f.target.WithContext(ctx).Clauses(clause.OnConflict{
				UpdateAll: true}).Create(&t).Error
		default:
			return err
		}
	case events.InconsistentEventTypeBaseMissing:
		return f.target.WithContext(ctx).
			Where("id = ?", evt.ID).Delete(&t).Error
	default:
		return errors.New("未知的不一致类型")
	}
}

// 一定要抓住，base 在校验时候的数据，到你修复的时候就变了
func (f *Fixer[T]) FixV2(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ:
		// 更新 target
		// 去 base 里面查出来，可能有几种情况，都会变的
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("超时或主动取消")
		case gorm.ErrRecordNotFound:
			// 变了，base 删除了，target 也要删除
			return f.target.WithContext(ctx).
				Where("id = ?", evt.ID).Delete(&t).Error
		case nil:
			return f.target.WithContext(ctx).
				Where("id = ?", evt.ID).Updates(&t).Error
		default:
			return err
		}
	case events.InconsistentEventTypeBaseMissing:
		return f.target.WithContext(ctx).
			Where("id = ?", evt.ID).Delete(&t).Error
	case events.InconsistentEventTypeTargetMissing:
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("超时或主动取消")
		case gorm.ErrRecordNotFound:
			return nil
		case nil:
			return f.target.WithContext(ctx).Create(&t).Error
		default:
			return err
		}
	default:
		return errors.New("未知的不一致类型")
	}
}
