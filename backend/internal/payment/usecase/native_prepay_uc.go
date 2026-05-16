package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type NativePrePaymentUC struct {
	repo domain.PaymentRepository
	l    logger.LoggerV1

	svc       domain.NativePayService
	appID     string
	mchID     string
	notifyURL string
}

func NewNativePrePaymentUC(
	repo domain.PaymentRepository,
	l logger.LoggerV1,
	svc domain.NativePayService,
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

	if err := ensurePaymentRecord(ctx, uc.repo, pmt); err != nil {
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
		uc.l.Error("原生预支付失败", logger.Error(err))
		return "", err
	}

	uc.l.Debug("原生预支付成功", logger.Field{Key: "codeURL", Value: codeURL})
	return codeURL, nil
}

type PrePaymentCmd struct {
	Amt         domain.Amount
	BizTradeNo  string
	Description string
}
