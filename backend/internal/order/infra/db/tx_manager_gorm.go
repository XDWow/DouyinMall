package db

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"gorm.io/gorm"
)

type txKey struct{}

// 渚濊禆 *gorm.DB 瀹炵幇浜嬪姟
type GormTxManager struct {
	db *gorm.DB
}

func NewGormTxManager(db *gorm.DB) domain.TxManager {
	return &GormTxManager{db: db}
}

func (m *GormTxManager) Tx(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func TxFromContext(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if !ok {
		panic("transaction not found in context")
	}
	return tx
}

func DBFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if ok && tx != nil {
		return tx
	}
	return fallback.WithContext(ctx)
}


