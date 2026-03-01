package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   "/var/log/agent.log",
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(lumberjackLogger),
		zapcore.DebugLevel,
	)

	l := zap.New(core, zap.AddCaller())
	return logger.NewZapLogger(l)
}
