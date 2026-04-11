package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	// 配置 Lumberjack 以支持日志文件滚动
	lumberjackLogger := &lumberjack.Logger{
		// 注意运行用户对日志目录的写权限
		Filename:   "/var/log/user.log", // 日志文件路径
		MaxSize:    50,                  // 单个日志文件最大体积（MB）
		MaxBackups: 3,                   // 最多保留的旧日志文件个数
		MaxAge:     28,                  // 旧日志文件最多保留天数
		Compress:   true,                // 是否压缩轮转后的旧日志
	}

	// 创建 zap 核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(lumberjackLogger),
		zapcore.DebugLevel, // 日志级别
	)
	l := zap.New(core, zap.AddCaller())
	res := logger.NewZapLogger(l)
	return res
}

