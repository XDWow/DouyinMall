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
	alipayadapter "github.com/XDWow/DouyinMall/backend/internal/payment/infra/alipay"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/wechat"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	ali "github.com/smartwalle/alipay/v3"
	"github.com/spf13/viper"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const (
	providerMockWechat = "mock_wechat"
	providerWechat     = "wechat"
	providerAlipay     = "alipay"
)

func InitNativePayService() domain.NativePayService {
	switch paymentProvider() {
	case providerMockWechat:
		return wechat.NewMockWechatNativeService(
			viper.GetString("wechat.api_base_url"),
			viper.GetString("wechat.mch_id"),
		)
	case providerWechat:
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
	case providerAlipay:
		return alipayadapter.NewNativeService(initAlipayClient())
	default:
		panic(fmt.Errorf("unsupported payment provider: %s", paymentProvider()))
	}
}

func InitNativePrePaymentUC(
	repo domain.PaymentRepository,
	l logger.LoggerV1,
	svc domain.NativePayService,
) *usecase.NativePrePaymentUC {
	appID, mchID, notifyURL := currentPayIdentity()
	return usecase.NewNativePrePaymentUC(repo, l, svc, appID, mchID, notifyURL)
}

func InitSyncPaymentOrderUC(
	svc domain.NativePayService,
	payCallbackUC *usecase.PayCallbackUC,
	l logger.LoggerV1,
) *usecase.SyncPaymentOrderUC {
	return usecase.NewSyncPaymentOrderUC(svc, payCallbackUC, l)
}

func InitPaymentProvider() string {
	return paymentProvider()
}

func InitAlipayClient() *ali.Client {
	if paymentProvider() != providerAlipay {
		return nil
	}
	return initAlipayClient()
}

func InitWechatNotifyHandler() *notify.Handler {
	if paymentProvider() != providerWechat {
		return nil
	}

	handler, err := notify.NewRSANotifyHandler(viper.GetString("wechat.api_v3_key"), nil)
	if err != nil {
		panic(fmt.Errorf("create wechat notify handler failed: %w", err))
	}
	return handler
}

func currentPayIdentity() (appID, mchID, notifyURL string) {
	switch paymentProvider() {
	case providerAlipay:
		return viper.GetString("alipay.app_id"), "", viper.GetString("alipay.notify_url")
	default:
		return viper.GetString("wechat.app_id"), viper.GetString("wechat.mch_id"), viper.GetString("wechat.notify_url")
	}
}

func paymentProvider() string {
	provider := strings.ToLower(strings.TrimSpace(viper.GetString("payment.provider")))
	if provider != "" {
		return provider
	}

	switch strings.ToLower(strings.TrimSpace(viper.GetString("payment.mode"))) {
	case "", "real":
		return providerWechat
	case "mock":
		return providerMockWechat
	default:
		return providerWechat
	}
}

func initAlipayClient() *ali.Client {
	appID := strings.TrimSpace(viper.GetString("alipay.app_id"))
	privateKey := strings.TrimSpace(viper.GetString("alipay.private_key"))
	publicKey := strings.TrimSpace(viper.GetString("alipay.public_key"))

	if appID == "" {
		panic("alipay.app_id is required when payment.provider=alipay")
	}
	if privateKey == "" {
		panic("alipay.private_key is required when payment.provider=alipay")
	}
	if publicKey == "" {
		panic("alipay.public_key is required when payment.provider=alipay")
	}

	client, err := ali.New(appID, privateKey, !viper.GetBool("alipay.sandbox"))
	if err != nil {
		panic(fmt.Errorf("init alipay client failed: %w", err))
	}
	if err = client.LoadAliPayPublicKey(publicKey); err != nil {
		panic(fmt.Errorf("load alipay public key failed: %w", err))
	}
	return client
}

func loadWechatPrivateKey(path string) *rsa.PrivateKey {
	if paymentProvider() == providerMockWechat {
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
