package ioc

import (
	"fmt"

	httptransport "github.com/XDWow/DouyinMall/backend/internal/search/transport/http"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitHTTPServer(handler *httptransport.Handler) *ginx.Server {
	port := viper.GetInt("http.server.port")
	if port == 0 {
		port = 18093
	}

	engine := gin.Default()
	handler.RegisterRoutes(engine)

	return &ginx.Server{
		Engine: engine,
		Addr:   fmt.Sprintf(":%d", port),
	}
}
