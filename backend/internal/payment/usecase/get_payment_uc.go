package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type GetPaymentUC struct {
	repo domain.PaymentRepository
	l    logger.LoggerV1
}

func NewGetPaymentUC(repo domain.PaymentRepository, l logger.LoggerV1) *GetPaymentUC {
	return &GetPaymentUC{repo, l}
}

type QueryPaymentCmd struct {
	BizTradeNo string // 业务侧支付单主键（与预下单时传入的商户单号一致）
}

func (uc *GetPaymentUC) Execute(ctx context.Context, cmd QueryPaymentCmd) (domain.Payment, error) {
	if cmd.BizTradeNo == "" {
		return domain.Payment{}, errors.New("业务交易号不能为空")
	}
	return uc.repo.GetPayment(ctx, cmd.BizTradeNo)
}
