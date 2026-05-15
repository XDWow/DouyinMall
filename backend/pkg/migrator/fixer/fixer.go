package fixer

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/pkg/migrator"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator/events"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Fixer[T migrator.Entity] struct {
	base    *gorm.DB
	target  *gorm.DB
	columns []string
}

func (f *Fixer[T]) Fix(ctx context.Context, evt events.InconsistentEvent) error {
	panic("not implemented")
}

// FixV1 resolves inconsistent records with an upsert-style repair strategy.
func (f *Fixer[T]) FixV1(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ, events.InconsistentEventTypeTargetMissing:
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("canceled or deadline exceeded")
		case gorm.ErrRecordNotFound:
			return f.target.WithContext(ctx).
				Where("id = ?", evt.ID).Delete(&t).Error
		case nil:
			return f.target.WithContext(ctx).Clauses(clause.OnConflict{
				UpdateAll: true,
			}).Create(&t).Error
		default:
			return err
		}
	case events.InconsistentEventTypeBaseMissing:
		return f.target.WithContext(ctx).
			Where("id = ?", evt.ID).Delete(&t).Error
	default:
		return errors.New("unsupported inconsistent event type")
	}
}

// FixV2 resolves inconsistent records with targeted update/create/delete operations.
func (f *Fixer[T]) FixV2(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ:
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("canceled or deadline exceeded")
		case gorm.ErrRecordNotFound:
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
			return errors.New("canceled or deadline exceeded")
		case gorm.ErrRecordNotFound:
			return nil
		case nil:
			return f.target.WithContext(ctx).Create(&t).Error
		default:
			return err
		}
	default:
		return errors.New("unsupported inconsistent event type")
	}
}
