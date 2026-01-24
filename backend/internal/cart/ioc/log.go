package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	// 配置Lumberjack以支持日志文件的滚动
	lumberjackLogger := &lumberjack.Logger{
		// 注意有没有权限
		Filename:   "/var/log/user.log", // 指定日志文件路径
		MaxSize:    50,                  // 每个日志文件的最大大小，单位：MB
		MaxBackups: 3,                   // 保留旧日志文件的最大个数
		MaxAge:     28,                  // 保留旧日志文件的最大天数
		Compress:   true,                // 是否压缩旧的日志文件
	}

	// 创建zap日志核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(lumberjackLogger),
		zapcore.DebugLevel, // 设置日志级别
	)
	l := zap.New(core, zap.AddCaller())
	res := logger.NewZapLogger(l)
	return res			
}