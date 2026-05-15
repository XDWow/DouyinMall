package ioc

import (
	"os"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// stdout 输出，Docker Desktop / docker logs 可见
	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	// 文件输出（容器内持久化）
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "/var/log/user.log",
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}),
		zapcore.DebugLevel,
	)

	l := zap.New(zapcore.NewTee(stdoutCore, fileCore), zap.AddCaller())
	return logger.NewZapLogger(l)
}
