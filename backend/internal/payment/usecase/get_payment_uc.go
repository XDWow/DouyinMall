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
	BizTradeNo string // 杩欎釜灏辨槸鏀粯浜ゆ槗鍗曠殑涓婚敭
}

func (uc *GetPaymentUC) Execute(ctx context.Context, cmd QueryPaymentCmd) (domain.Payment, error) {
	if cmd.BizTradeNo == "" {
		return domain.Payment{}, errors.New("涓氬姟浜ゆ槗鍙蜂负绌?)
	}
	return uc.repo.GetPayment(ctx, cmd.BizTradeNo)
}


