package ioc

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	httptransport "github.com/XDWow/DouyinMall/backend/internal/payment/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitHTTPServer(payCallbackUC *usecase.PayCallbackUC, l logger.LoggerV1) *ginx.Server {
	port := viper.GetInt("http.server.port")
	if port == 0 {
		port = 8093
	}

	certSerialNo := viper.GetString("wechat.cert_serial_no")
	apiV3Key := viper.GetString("wechat.api_v3_key")
	privateKey := maybeLoadPrivateKey(viper.GetString("wechat.private_key_path"))

	engine := gin.Default()
	callbackHandler := httptransport.NewWechatCallbackHandler(
		payCallbackUC,
		certSerialNo,
		privateKey,
		apiV3Key,
		l,
	)
	callbackHandler.RegisterRoutes(engine)

	return &ginx.Server{
		Engine: engine,
		Addr:   fmt.Sprintf(":%d", port),
	}
}

func maybeLoadPrivateKey(path string) *rsa.PrivateKey {
	if path == "" || strings.ToLower(viper.GetString("payment.mode")) == "mock" {
		return nil
	}
	return loadPrivateKey(path)
}

func loadPrivateKey(path string) *rsa.PrivateKey {
	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("read merchant private key failed: %w", err))
	}

	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		panic("decode merchant private key PEM failed")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(fmt.Errorf("parse merchant private key failed: %w", err))
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		panic("merchant private key is not RSA")
	}

	return rsaPrivateKey
}
