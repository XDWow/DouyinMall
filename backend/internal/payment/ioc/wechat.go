package ioc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/wechat"
	"os"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/spf13/viper"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// InitWechatNativeApiService 初始化微信 Native 支付 SDK
func InitWechatNativeApiService() *native.NativeApiService {
	// 读取配置
	mode := viper.GetString("payment.mode")
	mchID := viper.GetString("wechat.mch_id")
	certSerialNo := viper.GetString("wechat.cert_serial_no")
	privateKeyPath := viper.GetString("wechat.private_key_path")
	apiV3Key := viper.GetString("wechat.api_v3_key")

	var client *core.Client
	var err error

	if mode == "mock" {
		// Mock 模式：生成假私钥，跳过验证
		// 虽然不会真正调用微信 API，但 SDK 需要完整的初始化
		fakePrivateKey := generateFakePrivateKey()
		client, err = core.NewClient(
			context.Background(),
			option.WithWechatPayAutoAuthCipher(
				mchID,
				certSerialNo,
				fakePrivateKey,
				apiV3Key,
			),
		)
		// 注意：由于 wechatpay-go SDK 不支持自定义域名
		// Mock 模式建议改用 HTTP 客户端直接调用 Mock 服务
	} else {
		// 真实模式：加载完整证书
		privateKey := loadWechatPrivateKey(privateKeyPath)
		client, err = core.NewClient(
			context.Background(),
			option.WithWechatPayAutoAuthCipher(
				mchID,
				certSerialNo,
				privateKey,
				apiV3Key,
			),
		)
	}

	if err != nil {
		panic(fmt.Errorf("初始化微信支付客户端失败: %w", err))
	}

	// 创建 Native 支付服务
	return &native.NativeApiService{Client: client}
}

// InitWechatConfig 初始化微信支付配置
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

// WechatConfig 微信支付配置（仅在 ioc 内部使用）
type WechatConfig struct {
	AppID        string
	MchID        string
	NotifyURL    string
	CertSerialNo string
	APIv3Key     string
	KeyPath      string
}

// InitWechatAppID 提供微信 AppID
func InitWechatAppID() string {
	return viper.GetString("wechat.app_id")
}

// InitWechatMchID 提供微信商户号
func InitWechatMchID() string {
	return viper.GetString("wechat.mch_id")
}

// InitWechatNotifyURL 提供回调地址
func InitWechatNotifyURL() string {
	return viper.GetString("wechat.notify_url")
}

// InitWechatNativeService 初始化微信支付服务（包装 SDK）
func InitWechatNativeService(svc *native.NativeApiService) domain.WechatNativeService {
	return wechat.NewWechatNativeService(svc)
}

// InitNativePrePaymentUC 初始化预支付UC
func InitNativePrePaymentUC(
	repo domain.PaymentRepository,
	l logger.LoggerV1,
	wechatSvc domain.WechatNativeService,
) *usecase.NativePrePaymentUC {
	return usecase.NewNativePrePaymentUC(
		repo, l, wechatSvc,
		viper.GetString("wechat.app_id"),
		viper.GetString("wechat.mch_id"),
		viper.GetString("wechat.notify_url"),
	)
}

// InitSyncWechatOrderUC 初始化同步UC
func InitSyncWechatOrderUC(
	svc *native.NativeApiService,
	payCallbackUC *usecase.PayCallbackUC,
	l logger.LoggerV1,
) *usecase.SyncWechatOrderUC {
	return usecase.NewSyncWechatOrderUC(
		svc, payCallbackUC, l,
		viper.GetString("wechat.mch_id"),
	)
}

// loadWechatPrivateKey 加载微信商户私钥
func loadWechatPrivateKey(path string) *rsa.PrivateKey {
	// Mock 模式下可能不需要真实的私钥文件
	mode := viper.GetString("payment.mode")
	if mode == "mock" {
		// 返回一个空的私钥（Mock 模式不验证）
		return nil
	}

	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("读取微信商户私钥失败: %w", err))
	}

	privateKey, err := utils.LoadPrivateKeyWithPath(path)
	if err != nil {
		// 尝试从字节解析
		block, _ := pem.Decode(privateKeyBytes)
		if block == nil {
			panic("解析微信商户私钥 PEM 格式失败")
		}

		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			panic(fmt.Errorf("解析微信商户私钥失败: %w", err))
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			panic("私钥不是 RSA 格式")
		}
		return rsaKey
	}

	return privateKey
}

// generateFakePrivateKey 生成一个假的私钥用于 Mock 模式
func generateFakePrivateKey() *rsa.PrivateKey {
	// 使用固定的测试私钥（2048位）
	// 这只是用于 Mock 模式，不用于真实加密
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		// 如果生成失败，返回一个预定义的私钥
		// 在 Mock 模式下，由于跳过了验证，这个私钥不会被实际使用
		return &rsa.PrivateKey{}
	}
	return key
}
