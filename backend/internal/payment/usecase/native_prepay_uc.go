package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type NativePrePaymentUC struct {
	repo domain.PaymentRepository
	l    logger.LoggerV1

	svc       domain.WechatNativeService
	appID     string
	mchID     string
	notifyURL string
}

func NewNativePrePaymentUC(
	repo domain.PaymentRepository,
	l logger.LoggerV1,
	svc domain.WechatNativeService,
	appID, mchID, notifyURL string,
) *NativePrePaymentUC {
	return &NativePrePaymentUC{
		svc:       svc,
		repo:      repo,
		l:         l,
		appID:     appID,
		mchID:     mchID,
		notifyURL: notifyURL,
	}
}

func (uc *NativePrePaymentUC) Execute(ctx context.Context, cmd PrePaymentCmd) (string, error) {
	pmt := domain.Payment{
		Amt:         cmd.Amt,
		BizTradeNo:  cmd.BizTradeNo,
		Description: cmd.Description,
	}

	existing, err := uc.repo.GetPayment(ctx, cmd.BizTradeNo)
	switch {
	case err == nil:
		if existing.Status == domain.PaymentStatusSuccess {
			return "", domain.ErrPaymentAlreadyPaid
		}
		if existing.Amt.Total != cmd.Amt.Total || existing.Amt.Currency != cmd.Amt.Currency {
			return "", domain.ErrPaymentAmountChanged
		}
	case errors.Is(err, domain.ErrPaymentNotFound):
		if err = uc.repo.AddPayment(ctx, pmt); err != nil {
			return "", err
		}
	default:
		return "", err
	}

	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	timeExpire := time.Now().Add(30 * time.Minute).Unix()

	codeURL, err := uc.svc.Prepay(c, domain.PrepayRequest{
		AppID:       uc.appID,
		MchID:       uc.mchID,
		Description: pmt.Description,
		OutTradeNo:  pmt.BizTradeNo,
		NotifyURL:   uc.notifyURL,
		TimeExpire:  timeExpire,
		Amount:      pmt.Amt,
	})
	if err != nil {
		uc.l.Error("wechat prepay failed", logger.Error(err))
		return "", err
	}
	uc.l.Debug("wechat prepay success", logger.Field{Key: "codeURL", Value: codeURL})
	return codeURL, nil
}

type PrePaymentCmd struct {
	Amt         domain.Amount
	BizTradeNo  string // 业务方传，比如 orderID
	Description string
}
