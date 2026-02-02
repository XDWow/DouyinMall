package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// usecase（业务用例） 是职责更明确的 svc
// 预支付：拿扫码支付的二维码，同时存支付记录
type NativePrePaymentUC struct {
	repo domain.PaymentRepository
	l    logger.LoggerV1

	svc       domain.WechatNativeService // 改为接口，方便测试使用mock实现
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
	// 唯一索引冲突
	// 业务方唤起了支付，但是没付，下一次再过来，应该换 BizTradeNO
	// 要做的：
	// 1、存支付记录
	// 2、调用微信 api 获取 code_url
	pmt := domain.Payment{
		Amt:         cmd.Amt,
		BizTradeNo:  cmd.BizTradeNo,
		Description: cmd.Description,
	}
	err := uc.repo.AddPayment(ctx, pmt)
	if err != nil {
		return "", err
	}
	c, cancel := context.WithTimeout(ctx, time.Second*3) // 调微信api，我决定花3s
	defer cancel()
	
	// 设置30分钟有效
	timeExpire := time.Now().Add(time.Minute * 30).Unix()
	
	codeURL, err := uc.svc.Prepay(c, domain.PrepayRequest{
		// 标识app，谁在收钱，这里要注册，合法的App
		AppID: uc.appID,
		MchID: uc.mchID,
		// 订单描述，用户在微信支付页面看到的内容
		Description: pmt.Description,
		// 这个地方是有讲究的
		// 选择1：业务方直接给我，我透传，我啥也不干（推荐）
		// 选择2：业务方给我它的业务标识，我自己生成一个 - 担忧出现重复
		// 注意，不管你是选择 1 还是选择 2，业务方都一定要传给来一个唯一标识
		// Biz + BizTradeNo 唯一， biz + biz_id
		// 这笔支付交易的标识，主键
		OutTradeNo: pmt.BizTradeNo,
		NotifyURL:  uc.notifyURL,
		// 设置30分钟有效
		TimeExpire: timeExpire,
		Amount:     pmt.Amt,
	})
	if err != nil {
		uc.l.Error("微信 prepay 失败", logger.Error(err))
		return "", err
	}
	uc.l.Debug("微信 prepay 成功", logger.Field{Key: "codeURL", Value: codeURL})
	return codeURL, nil
}

type PrePaymentCmd struct {
	Amt         domain.Amount
	BizTradeNo  string
	Description string
}
