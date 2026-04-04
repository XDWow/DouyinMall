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

	// stdout 杈撳嚭锛圖ocker Desktop / docker logs 鍙锛?
	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	// 鏂囦欢杈撳嚭锛堝鍣ㄥ唴鎸佷箙鍖栵級
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


