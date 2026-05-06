package ioc

import (
	"net/http"

	"github.com/XDWow/DouyinMall/backend/internal/bff/handler"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	ginxratelimit "github.com/XDWow/DouyinMall/backend/pkg/ginx/middlewares/ratelimit"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitGinServer(
	agentHandler *handler.AgentHandler,
	authHandler *handler.AuthHandler,
	limiter ratelimit.Limiter,
) *ginx.Server {
	engine := gin.Default()

	engine.Use(ginxratelimit.NewBuilder(limiter).Prefix("bff").Build())
	engine.Use(corsMiddleware())

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Msg: "ok"})
	})

	authGroup := engine.Group("/api/auth")
	authHandler.RegisterRoutes(authGroup)

	agentGroup := engine.Group("/api/agent", ginx.RequireGatewayUser())
	agentHandler.RegisterRoutes(agentGroup)

	addr := viper.GetString("http.addr")
	if addr == "" {
		addr = ":8080"
	}
	return &ginx.Server{Engine: engine, Addr: addr}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}


