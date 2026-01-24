package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"time"
)

type CreateOrderUseCase struct {
	repo domain.OrderRepository
	log  logger.LoggerV1
}

// cmd 为什么要多这一层：
// 1. proto 是协议输入，来源不可信；
// 2. domain 是业务事实，存在即合法；
// 3. 校验与决策属于 usecase，而不是 handler 或 domain；
//
// 因此调用链为：
//
//	handler: proto -> cmd
//	usecase: cmd -> domain
//
// 通过引入 cmd，
// 将“记得校验 domain 合法性”的流程纪律，
// 转换为“domain 类型即业务事实，一定合法”的类型级约束。
//
// 好处是：
// - 多入口（HTTP / gRPC / MQ / Job）可复用 usecase；
// - 公共代码抽取不会引入非法 domain；
// - 开发者看到 domain 就可以确信其合法性。
//
// 总结：
// cmd 是业务的统一入口命令
// 所有入口只负责翻译
// 所有校验和决策（业务相关）都集中在 usecase
// domain 只承载已经成立的业务事实
type CreateOrderCmd struct {
	UserID   int64
	Currency string
	Phone    string
	Address  domain.Address
	Items    []domain.OrderItem
}

func NewCreateOrderUseCase(
	repo domain.OrderRepository,
	log logger.LoggerV1,
) *CreateOrderUseCase {
	return &CreateOrderUseCase{repo: repo, log: log}
}

func (uc *CreateOrderUseCase) Execute(
	ctx context.Context,
	cmd CreateOrderCmd,
) (int64, error) {
	// 校验并转换
	if cmd.UserID <= 0 {
		return 0, errors.New("无效用户，创建订单失败")
	}
	if len(cmd.Items) == 0 {
		return 0, errors.New("订单为空，创建失败")
	}
	order := orderDomainToCmd(cmd)

	if err := uc.repo.Save(ctx, &order); err != nil {
		uc.log.Error("保存订单失败", logger.Error(err))
		return 0, err
	}

	return order.ID, nil
}

func orderDomainToCmd(cmd CreateOrderCmd) domain.Order {
	return domain.Order{
		UserID: cmd.UserID,
		Phone:  cmd.Phone,
		Amt: domain.Amount{
			Currency: cmd.Currency,
		},
		Addr:       cmd.Address,
		OrderItems: cmd.Items,
		ExpireAt:   time.Now().Add(30 * time.Minute),
	}
}
