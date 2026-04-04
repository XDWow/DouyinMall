package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"go.uber.org/zap"
)

func InitLogger() logger.LoggerV1 {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return logger.NewZapLogger(l)
}


