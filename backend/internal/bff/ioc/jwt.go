package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/jwtx"
	"github.com/spf13/viper"
)

func InitJWTManager() *jwtx.JWTManager {
	accessSecret := viper.GetString("jwt.access_secret")
	refreshSecret := viper.GetString("jwt.refresh_secret")

	accessExpiry := viper.GetDuration("jwt.access_expiry")
	if accessExpiry == 0 {
		accessExpiry = 2 * time.Hour
	}
	refreshExpiry := viper.GetDuration("jwt.refresh_expiry")
	if refreshExpiry == 0 {
		refreshExpiry = 7 * 24 * time.Hour
	}

	return jwtx.NewJWTManager(accessSecret, refreshSecret, accessExpiry, refreshExpiry)
}
