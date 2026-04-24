package ioc

import (
	"fmt"

	httptransport "github.com/XDWow/DouyinMall/backend/internal/payment/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	ali "github.com/smartwalle/alipay/v3"
	"github.com/spf13/viper"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

func InitHTTPServer(
	payCallbackUC *usecase.PayCallbackUC,
	l logger.LoggerV1,
	provider string,
	alipayClient *ali.Client,
	wechatNotifyHandler *notify.Handler,
) *ginx.Server {
	port := viper.GetInt("http.server.port")
	if port == 0 {
		port = 8093
	}

	engine := gin.Default()
	callbackHandler := httptransport.NewPaymentCallbackHandler(
		payCallbackUC,
		wechatNotifyHandler,
		alipayClient,
		provider,
		l,
	)
	callbackHandler.RegisterRoutes(engine)

	return &ginx.Server{
		Engine: engine,
		Addr:   fmt.Sprintf(":%d", port),
	}
}
