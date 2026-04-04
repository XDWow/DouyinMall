package connpool

import (
	"context"
	"database/sql"
	"errors"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"gorm.io/gorm"
)

type DoubleWritePool struct {
	src     gorm.ConnPool
	dst     gorm.ConnPool
	pattern *atomicx.Value[string]
	l       logger.LoggerV1
}

func NewDoubleWritePool(src, dst gorm.ConnPool, pattern string) *DoubleWritePool {
	return &DoubleWritePool{
		src:     src,
		dst:     dst,
		pattern: atomicx.NewValueOf(pattern),
	}
}

type DoubleWritePoolTx struct {
	src     *sql.Tx
	dst     *sql.Tx
	pattern string
	l       logger.LoggerV1
}

func (d *DoubleWritePoolTx) Commit() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Commit()
	case PatternDstOnly:
		return d.dst.Commit()
	case PatternSrcFirst:
		err := d.src.Commit()
		if err != nil {
			return err
		}
		if d.dst != nil {
			err = d.dst.Commit()
			if err != nil {
				// 渚濇棫绠椾綘鎴愬姛锛屾墦涓棩蹇楀嵆鍙?
				d.l.Error("dst 浜嬪姟鎻愪氦澶辫触")
			}
		}
		return nil
	case PatternDstFirst:
		err := d.dst.Commit()
		if err != nil {
			return err
		}
		if d.src != nil {
			err = d.src.Commit()
			if err != nil {
				// 渚濇棫绠椾綘鎴愬姛锛屾墦涓棩蹇楀嵆鍙?
				d.l.Error("src 浜嬪姟鎻愪氦澶辫触")
			}
		}
		return nil
	default:
		return errors.New("鏈煡妯″紡")
	}
}

func (d *DoubleWritePoolTx) Rollback() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Rollback()
	case PatternDstOnly:
		return d.dst.Rollback()
	case PatternSrcFirst:
		err := d.src.Rollback()
		if err != nil {
			return err
		}
		if d.dst != nil {
			err = d.dst.Rollback()
			if err != nil {
				// 渚濇棫绠椾綘鎴愬姛锛屾墦涓棩蹇楀嵆鍙?
				d.l.Error("dst 浜嬪姟鍥炴粴澶辫触")
			}
		}
		return nil
	case PatternDstFirst:
		err := d.dst.Commit()
		if err != nil {
			return err
		}
		if d.src != nil {
			err = d.src.Commit()
			if err != nil {
				// 渚濇棫绠椾綘鎴愬姛锛屾墦涓棩蹇楀嵆鍙?
				d.l.Error("src 浜嬪姟鍥炴粴澶辫触")
			}
		}
		return nil
	default:
		return errors.New("鏈煡妯″紡")
	}
}

func (d *DoubleWritePoolTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("implement me")
}

func (d *DoubleWritePoolTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.ExecContext(ctx, query, args...)
	case PatternSrcFirst:
		res, err := d.src.ExecContext(ctx, query, args...)
		if err != nil {
			return res, err
		}
		if d.dst == nil {
			return res, err
		}
		_, err = d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			// 璁版棩蹇?
			// dst 鍐欏け璐ワ紝涓嶈璁や负鏄け璐?
		}
		return res, err
	case PatternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case PatternDstFirst:
		res, err := d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			return res, err
		}
		if d.src == nil {
			return res, err
		}
		_, err = d.src.ExecContext(ctx, query, args...)
		if err != nil {
			// 璁版棩蹇?
			// dst 鍐欏け璐ワ紝涓嶈璁や负鏄け璐?
		}
		return res, err
	default:
		panic("鏈煡鐨勫弻鍐欐ā寮?)
		//return nil, errors.New("鏈煡鐨勫弻鍐欐ā寮?)
	}
}

func (d *DoubleWritePoolTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("鏈煡鐨勫弻鍐欐ā寮?)
		//return nil, errors.New("鏈煡鐨勫弻鍐欐ā寮?)
	}
}

func (d *DoubleWritePoolTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		panic("鏈煡鐨勫弻鍐欐ā寮?)
		//return nil, errors.New("鏈煡鐨勫弻鍐欐ā寮?)
	}
}

func (d *DoubleWritePool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	pattern := d.pattern.Load()
	switch pattern {
	case PatternSrcOnly:
		tx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		return &DoubleWritePoolTx{
			src:     tx,
			pattern: pattern,
		}, err
	case PatternSrcFirst:
		srcTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		dstTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			d.l.Error("dstTx 寮€鍚け璐?)
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
		}, nil //杩欓噷涓嶈兘浼爀rr锛屼笉瑕佽浠庡簱鐨勫け璐ュ奖鍝嶄富搴?
	case PatternDstOnly:
		tx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		return &DoubleWritePoolTx{
			src:     tx,
			pattern: pattern,
		}, err
	case PatternDstFirst:
		srcTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		dstTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			d.l.Error("srcTx 寮€鍚け璐?)
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
		}, nil //杩欓噷涓嶈兘浼爀rr锛屼笉瑕佽浠庡簱鐨勫け璐ュ奖鍝嶄富搴?
	default:
		return nil, errors.New("鏈煡鐨勫弻鍐欐ā寮?)
	}
}

// PrepareContext Prepare(棰勭紪璇? 鐨勮鍙ヤ細杩涙潵杩欓噷
func (d *DoubleWritePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	//TODO implement me
	panic("implement me")
}

// 澧炲垹鏀癸紙鍐欙級璇彞杩涙潵杩欓噷
func (d *DoubleWritePool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	switch d.pattern.Load() {
	case PatternSrcOnly:
		return d.src.ExecContext(ctx, query, args...)
	case PatternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case PatternSrcFirst:
		_, err := d.src.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		return d.dst.ExecContext(ctx, query, args...)
	case PatternDstFirst:
		_, err := d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		return d.src.ExecContext(ctx, query, args...)
	default:
		panic("鏃犳晥妯″紡")
	}
}

func (d *DoubleWritePool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("鏃犳晥妯″紡")
		//return nil, errors.New("鏃犳晥妯″紡")
	}
}

func (d *DoubleWritePool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		// 瀹氫箟鍦ㄥ閮ㄧ殑缁撴瀯浣擄紝涓嶈兘鐩存帴鏋勯€狅紝鍙兘閫氳繃瀹冪粰浣犳彁渚涘垵濮嬪寲鏂规硶
		//return &sql.Row{
		//	err:  errors.New("鏃犳晥妯″紡"),
		//	rows: nil,
		//}

		// 閭ｆ€庝箞鍔烇紵璧板埌杩欓噷鑲畾鏄唬鐮侀敊璇紝鐩存帴閫氳繃 panic 鍛婄煡閿欒淇℃伅
		// 鍚岀悊涓轰簡涓€鑷存€э紝骞朵笖涓婇潰鐨勪篃涓嶇敤杩斿洖閿欒锛屽洜涓烘病鎰忎箟锛屽悗缁病娉曞鐞?
		panic("鏃犳晥妯″紡")
	}
}

func (d *DoubleWritePool) UpdatePattern(pattern string) {
	d.pattern.Store(pattern)
	// 鎴戣兘涓嶈兘锛屾湁浜嬪姟鏈彁浜ょ殑鎯呭喌涓嬶紝鎴戠姝慨鏀?
	// 鑳斤紝浣嗘槸鎬ц兘闂姣旇緝涓ラ噸锛屼綘闇€瑕佺淮鎸佷綇涓€涓凡寮€浜嬪姟鐨勮鏁帮紝骞跺湪杩欓噷妫€鏌ュ仛鏌愪簨锛岃鐢ㄩ攣浜?
}

const (
	PatternDstOnly  = "DST_ONLY"
	PatternSrcOnly  = "SRC_ONLY"
	PatternDstFirst = "DST_FIRST"
	PatternSrcFirst = "SRC_FIRST"
)


