package ioc

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/XDWow/DouyinMall/backend/internal/payment/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// InitHTTPServer 初始化 HTTP 服务器（用于接收微信回调）
func InitHTTPServer(
	payCallbackUC *usecase.PayCallbackUC,
	l logger.LoggerV1,
) *ginx.Server {
	// 读取配置
	port := viper.GetInt("http.server.port")
	if port == 0 {
		port = 8093 // 默认端口
	}

	// 读取微信支付配置
	certSerialNo := viper.GetString("wechat.cert_serial_no")
	privateKeyPath := viper.GetString("wechat.private_key_path")
	apiV3Key := viper.GetString("wechat.api_v3_key")

	// 加载商户私钥
	privateKey := loadPrivateKey(privateKeyPath)

	// 创建 Gin 引擎
	engine := gin.Default()

	// 创建回调处理器
	callbackHandler := http.NewWechatCallbackHandler(
		payCallbackUC,
		certSerialNo,
		privateKey,
		apiV3Key,
		l,
	)

	// 注册路由
	callbackHandler.RegisterRoutes(engine)

	return &ginx.Server{
		Engine: engine,
		Addr:   fmt.Sprintf(":%d", port),
	}
}

// loadPrivateKey 加载商户私钥
func loadPrivateKey(path string) *rsa.PrivateKey {
	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("读取商户私钥失败: %w", err))
	}

	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		panic("解析商户私钥 PEM 格式失败")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(fmt.Errorf("解析商户私钥失败: %w", err))
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		panic("私钥不是 RSA 格式")
	}

	return rsaPrivateKey
}
