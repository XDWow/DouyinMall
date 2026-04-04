package ioc

import (
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() logger.LoggerV1 {
	// 閰嶇疆Lumberjack浠ユ敮鎸佹棩蹇楁枃浠剁殑婊氬姩
	lumberjackLogger := &lumberjack.Logger{
		// 娉ㄦ剰鏈夋病鏈夋潈闄?
		Filename:   "/var/log/search.log", // 鎸囧畾鏃ュ織鏂囦欢璺緞
		MaxSize:    50,                    // 姣忎釜鏃ュ織鏂囦欢鐨勬渶澶уぇ灏忥紝鍗曚綅锛歁B
		MaxBackups: 3,                     // 淇濈暀鏃ф棩蹇楁枃浠剁殑鏈€澶т釜鏁?
		MaxAge:     28,                    // 淇濈暀鏃ф棩蹇楁枃浠剁殑鏈€澶уぉ鏁?
		Compress:   true,                  // 鏄惁鍘嬬缉鏃х殑鏃ュ織鏂囦欢
	}

	// 鍒涘缓zap鏃ュ織鏍稿績
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(lumberjackLogger),
		zapcore.DebugLevel, // 璁剧疆鏃ュ織绾у埆
	)
	l := zap.New(core, zap.AddCaller())
	res := logger.NewZapLogger(l)
	return res
}


