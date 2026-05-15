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
				// 依然认为成功，打个日志即可
				d.l.Error("dst 事务提交失败")
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
				// 依然认为成功，打个日志即可
				d.l.Error("src 事务提交失败")
			}
		}
		return nil
	default:
		return errors.New("未知模式")
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
				// 依然认为成功，打个日志即可
				d.l.Error("dst 事务回滚失败")
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
				// 依然认为成功，打个日志即可
				d.l.Error("src 事务回滚失败")
			}
		}
		return nil
	default:
		return errors.New("未知模式")
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
			// 记录日志
			// dst 写失败，不认为是失败
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
			// 记录日志
			// dst 写失败，不认为是失败
		}
		return res, err
	default:
		panic("未知双写模式")
	}
}

func (d *DoubleWritePoolTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("未知双写模式")
	}
}

func (d *DoubleWritePoolTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		panic("未知双写模式")
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
			d.l.Error("dstTx 开启失败")
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
		}, nil // 这里不能返回 err，不要让从库的失败影响主库
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
			d.l.Error("srcTx 开启失败")
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
		}, nil // 这里不能返回 err，不要让从库的失败影响主库
	default:
		return nil, errors.New("未知双写模式")
	}
}

// PrepareContext Prepare(预编译) 的语句会进来这里
func (d *DoubleWritePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	//TODO implement me
	panic("implement me")
}

// 增删改（写）语句进来这里
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
		panic("无效模式")
	}
}

func (d *DoubleWritePool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("无效模式")
	}
}

func (d *DoubleWritePool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	switch d.pattern.Load() {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		// 定义在外部的结构体，不能直接构造，只能通过它给你提供初始化方法
		//return &sql.Row{
		//	err:  errors.New("无效模式"),
		//	rows: nil,
		//}

		// 走到这里肯定是代码错误，直接通过 panic 告知错误信息
		// 同理为了保持一致性，上面的也不用返回错误，因为没意义，后续没法处理
		panic("无效模式")
	}
}

func (d *DoubleWritePool) UpdatePattern(pattern string) {
	d.pattern.Store(pattern)
	// 我能不能在有事务未提交的情况下禁止修改？
	// 能，但性能问题比较严重，你需要维护一个已开事务的计数，并在这里检查做某件事，要用锁
}

const (
	PatternDstOnly  = "DST_ONLY"
	PatternSrcOnly  = "SRC_ONLY"
	PatternDstFirst = "DST_FIRST"
	PatternSrcFirst = "SRC_FIRST"
)