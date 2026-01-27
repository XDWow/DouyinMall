package domain

import "context"

// TxManager 事务管理器接口，用于处理数据库事务
// 放在domain层而不是usecase层，避免循环依赖
type TxManager interface {
	// Tx 在事务中执行操作
	// 如果fn返回error，事务会回滚；否则提交
	Tx(ctx context.Context, fn func(context.Context) error) error
}
