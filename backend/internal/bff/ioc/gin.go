package ioc

import (
	"net/http"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/bff/handler"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	ginxratelimit "github.com/XDWow/DouyinMall/backend/pkg/ginx/middlewares/ratelimit"
	"github.com/XDWow/DouyinMall/backend/pkg/jwtx"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitGinServer(
	agentHandler *handler.AgentHandler,
	authHandler *handler.AuthHandler,
	tradeHandler *handler.TradeHandler,
	jwtMgr *jwtx.JWTManager,
	limiter ratelimit.Limiter,
) *ginx.Server {
	engine := gin.Default()

	engine.Use(ginxratelimit.NewBuilder(limiter).Prefix("bff").Build())
	engine.Use(corsMiddleware())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Msg: "ok"})
	})

	authGroup := engine.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	tradeGroup := engine.Group("/trade")
	tradeHandler.RegisterRoutes(tradeGroup)

	agentGroup := engine.Group("/agent/api", jwtAuthMiddleware(jwtMgr))
	agentHandler.RegisterRoutes(agentGroup)

	addr := viper.GetString("http.addr")
	if addr == "" {
		addr = ":8080"
	}
	return &ginx.Server{Engine: engine, Addr: addr}
}

func jwtAuthMiddleware(mgr *jwtx.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "unauthorized"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "invalid authorization header"})
			return
		}

		claims, err := mgr.ParseAccessToken(token)
		if err != nil {
			if err == jwtx.ErrTokenExpired {
				c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 401, Msg: "token expired"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "invalid token"})
			return
		}

		c.Set("claims", &ginx.UserClaims{
			Id:               claims.UserID,
			RegisteredClaims: claims.RegisteredClaims,
		})
		c.Next()
	}
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
