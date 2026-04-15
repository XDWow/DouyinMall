package domain

import "context"

// TxManager 事务管理器，用于包裹数据库事务。
// 放在 domain 层而非 usecase，避免循环依赖。
type TxManager interface {
	// Tx 在事务中执行 fn；fn 返回 error 则回滚，否则提交。
	Tx(ctx context.Context, fn func(context.Context) error) error
}
