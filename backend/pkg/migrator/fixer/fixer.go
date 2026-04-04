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

// 鏈€涓€浜嗙櫨浜嗙殑鍐欐硶
// 涓嶇涓変竷浜屽崄涓€锛屾垜TM鐩存帴瑕嗙洊
// 鎶?event 褰撴垚涓€涓Е鍙戝櫒锛屼笉渚濊禆鐨?event 鐨勫叿浣撳唴瀹癸紙ID 蹇呴』涓嶅彲鍙橈級
// 淇杩欓噷锛屼篃鏀规垚鎵归噺锛燂紵
func (f *Fixer[T]) Fix(ctx context.Context, evt events.InconsistentEvent) error {
	panic("瀹炵幇鎴?)
}

// 鏇寸畝娲侊紝骞朵笖upsert璇彞锛岃В鍐充簡骞跺彂闂
// 鏍￠獙鐨勬椂鍊橳argetMissing锛屽鏋滃埌淇鏃跺凡缁忔湁浜嗭紝鍐嶆彃鍏ヨ偗瀹氬け璐ワ紝浣唘psert鑳芥垚鍔?
func (f *Fixer[T]) FixV1(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ,
		events.InconsistentEventTypeTargetMissing:
		// 鏇存柊 target
		// 鍘?base 閲岄潰鏌ュ嚭鏉ワ紝鍙兘鏈夊嚑绉嶆儏鍐碉紝閮戒細鍙樼殑
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("瓒呮椂鎴栦富鍔ㄥ彇娑?)
		case gorm.ErrRecordNotFound:
			// 鍙樹簡锛宐ase 鍒犻櫎浜嗭紝target 涔熻鍒犻櫎
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
		return errors.New("鏈煡鐨勪笉涓€鑷寸被鍨?)
	}
}

// 涓€瀹氳鎶撲綇锛宐ase 鍦ㄦ牎楠屾椂鍊欑殑鏁版嵁锛屽埌浣犱慨澶嶇殑鏃跺€欏氨鍙樹簡
func (f *Fixer[T]) FixV2(ctx context.Context, evt events.InconsistentEvent) error {
	var t T
	switch evt.Type {
	case events.InconsistentEventTypeNEQ:
		// 鏇存柊 target
		// 鍘?base 閲岄潰鏌ュ嚭鏉ワ紝鍙兘鏈夊嚑绉嶆儏鍐碉紝閮戒細鍙樼殑
		err := f.base.WithContext(ctx).
			Where("id = ?", evt.ID).First(&t).Error
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return errors.New("瓒呮椂鎴栦富鍔ㄥ彇娑?)
		case gorm.ErrRecordNotFound:
			// 鍙樹簡锛宐ase 鍒犻櫎浜嗭紝target 涔熻鍒犻櫎
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
			return errors.New("瓒呮椂鎴栦富鍔ㄥ彇娑?)
		case gorm.ErrRecordNotFound:
			return nil
		case nil:
			return f.target.WithContext(ctx).Create(&t).Error
		default:
			return err
		}
	default:
		return errors.New("鏈煡鐨勪笉涓€鑷寸被鍨?)
	}
}


