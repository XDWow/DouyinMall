package ioc

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/wechat"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/spf13/viper"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

func InitWechatNativeApiService() domain.WechatNativeService {
	if strings.EqualFold(viper.GetString("payment.mode"), "mock") {
		return wechat.NewMockWechatNativeService(
			viper.GetString("wechat.api_base_url"),
			viper.GetString("wechat.mch_id"),
		)
	}

	client, err := core.NewClient(
		context.Background(),
		option.WithWechatPayAutoAuthCipher(
			viper.GetString("wechat.mch_id"),
			viper.GetString("wechat.cert_serial_no"),
			loadWechatPrivateKey(viper.GetString("wechat.private_key_path")),
			viper.GetString("wechat.api_v3_key"),
		),
	)
	if err != nil {
		panic(fmt.Errorf("init wechat client failed: %w", err))
	}

	return wechat.NewWechatNativeService(&native.NativeApiService{Client: client})
}

func InitWechatConfig() WechatConfig {
	return WechatConfig{
		AppID:        viper.GetString("wechat.app_id"),
		MchID:        viper.GetString("wechat.mch_id"),
		NotifyURL:    viper.GetString("wechat.notify_url"),
		CertSerialNo: viper.GetString("wechat.cert_serial_no"),
		APIv3Key:     viper.GetString("wechat.api_v3_key"),
		KeyPath:      viper.GetString("wechat.private_key_path"),
	}
}

type WechatConfig struct {
	AppID        string
	MchID        string
	NotifyURL    string
	CertSerialNo string
	APIv3Key     string
	KeyPath      string
}

func InitWechatAppID() string {
	return viper.GetString("wechat.app_id")
}

func InitWechatMchID() string {
	return viper.GetString("wechat.mch_id")
}

func InitWechatNotifyURL() string {
	return viper.GetString("wechat.notify_url")
}

func InitWechatNativeService(svc domain.WechatNativeService) domain.WechatNativeService {
	return svc
}

func InitNativePrePaymentUC(
	repo domain.PaymentRepository,
	l logger.LoggerV1,
	wechatSvc domain.WechatNativeService,
) *usecase.NativePrePaymentUC {
	return usecase.NewNativePrePaymentUC(
		repo,
		l,
		wechatSvc,
		viper.GetString("wechat.app_id"),
		viper.GetString("wechat.mch_id"),
		viper.GetString("wechat.notify_url"),
	)
}

func InitSyncWechatOrderUC(
	svc domain.WechatNativeService,
	payCallbackUC *usecase.PayCallbackUC,
	l logger.LoggerV1,
) *usecase.SyncWechatOrderUC {
	return usecase.NewSyncWechatOrderUC(svc, payCallbackUC, l)
}

func loadWechatPrivateKey(path string) *rsa.PrivateKey {
	if strings.EqualFold(viper.GetString("payment.mode"), "mock") {
		return nil
	}

	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("read wechat private key failed: %w", err))
	}

	privateKey, err := utils.LoadPrivateKeyWithPath(path)
	if err == nil {
		return privateKey
	}

	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		panic("decode wechat private key PEM failed")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(fmt.Errorf("parse wechat private key failed: %w", err))
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		panic("wechat private key is not RSA")
	}
	return rsaKey
}


