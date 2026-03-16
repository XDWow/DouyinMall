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

// InitGinServer 初始化 gin HTTP 服务（BFF 网关）
func InitGinServer(agentHandler *handler.AgentHandler, authHandler *handler.AuthHandler, jwtMgr *jwtx.JWTManager, limiter ratelimit.Limiter) *ginx.Server {
	engine := gin.Default()

	// 全局限流（按 IP，滑动窗口）
	engine.Use(ginxratelimit.NewBuilder(limiter).Prefix("bff").Build())

	// CORS
	engine.Use(corsMiddleware())

	// 公开路由
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Msg: "ok"})
	})
	authGroup := engine.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	// Agent API（需要 JWT 认证）
	agentGroup := engine.Group("/agent/api", jwtAuthMiddleware(jwtMgr))
	agentHandler.RegisterRoutes(agentGroup)

	addr := viper.GetString("http.addr")
	if addr == "" {
		addr = ":8080"
	}
	return &ginx.Server{Engine: engine, Addr: addr}
}

// jwtAuthMiddleware 校验 access token，过期时返回 401 + code=401 供前端自动刷新
func jwtAuthMiddleware(mgr *jwtx.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "未登录"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "认证格式错误"})
			return
		}

		claims, err := mgr.ParseAccessToken(token)
		if err != nil {
			msg := "token 无效，请重新登录"
			if err == jwtx.ErrTokenExpired {
				// code=401 专门给前端识别"需要刷新"
				c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 401, Msg: "token 已过期"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: msg})
			return
		}

		c.Set("claims", &ginx.UserClaims{
			Id:               claims.UserID,
			RegisteredClaims: claims.RegisteredClaims,
		})
		c.Next()
	}
}

// corsMiddleware 简单 CORS 中间件
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
