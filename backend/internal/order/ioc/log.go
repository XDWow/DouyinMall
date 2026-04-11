package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	// Lumberjack：日志文件滚动
	lumberjackLogger := &lumberjack.Logger{
		// 注意进程对日志目录的写权限
		Filename:   "/var/log/user.log", // 日志文件路径
		MaxSize:    50,                  // 单文件最大 MB
		MaxBackups: 3,                   // 最多保留旧文件个数
		MaxAge:     28,                  // 保留天数
		Compress:   true,                // 是否压缩历史文件
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(lumberjackLogger),
		zapcore.DebugLevel, // 日志级别
	)
	l := zap.New(core, zap.AddCaller())
	return logger.NewZapLogger(l)
}
