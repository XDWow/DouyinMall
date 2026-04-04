//go:build legacy_agent

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

	// stdout 鏉堟挸鍤敍鍦杘cker Desktop / docker logs 閸欘垵顫嗛敍?
	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	// 閺傚洣娆㈡潏鎾冲毉閿涘牆顔愰崳銊ュ敶閹镐椒绠欓崠鏍电礆
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "/var/log/agent.log",
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


