package usecase

import "context"

// 事务接口，原则：接口靠近使用者，放在 usecase（使用事务）近的地方
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
