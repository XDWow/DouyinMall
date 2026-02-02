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
	BizTradeNo string // 这个就是支付交易单的主键
}

func (uc *GetPaymentUC) Execute(ctx context.Context, cmd QueryPaymentCmd) (domain.Payment, error) {
	if cmd.BizTradeNo == "" {
		return domain.Payment{}, errors.New("业务交易号为空")
	}
	return uc.repo.GetPayment(ctx, cmd.BizTradeNo)
}
