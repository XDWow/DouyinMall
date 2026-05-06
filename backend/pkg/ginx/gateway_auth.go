package ginx

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const GatewayUserIDKey = "gateway_user_id"

func RequireGatewayUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GatewayUserID(c)
		if userID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Result{Code: 4, Msg: "unauthorized"})
			return
		}
		c.Set(GatewayUserIDKey, userID)
		c.Next()
	}
}

func GatewayUserID(c *gin.Context) int64 {
	if value, ok := c.Get(GatewayUserIDKey); ok {
		switch typed := value.(type) {
		case int64:
			return typed
		case int:
			return int64(typed)
		}
	}

	if claims, ok := c.Get("claims"); ok {
		if uc, ok := claims.(*UserClaims); ok && uc.Id > 0 {
			return uc.Id
		}
	}

	for _, key := range []string{"X-User-ID", "X-User-Id"} {
		if userID := parsePositiveInt64(c.GetHeader(key)); userID > 0 {
			return userID
		}
	}

	if userID := parseBearerToken(c.GetHeader("Authorization")); userID > 0 {
		return userID
	}
	return 0
}

func parsePositiveInt64(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

type bearerClaims struct {
	UserID    int64  `json:"user_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func parseBearerToken(header string) int64 {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return 0
	}

	secret := strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET"))
	if secret == "" {
		return 0
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if tokenString == "" {
		return 0
	}

	token, err := jwt.ParseWithClaims(tokenString, &bearerClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return 0
	}

	claims, ok := token.Claims.(*bearerClaims)
	if !ok || !token.Valid || claims.TokenType != "access" || claims.UserID <= 0 {
		return 0
	}
	return claims.UserID
}
