package logger

// 一个空的实现：例子
type NopLogger struct {
}

func NewNoOpLogger() *NopLogger {
	return &NopLogger{}
}

func (n *NopLogger) Debug(msg string, args ...Field) {
}

func (n *NopLogger) Info(msg string, args ...Field) {
}

func (n *NopLogger) Warn(msg string, args ...Field) {
}

func (n *NopLogger) Error(msg string, args ...Field) {
}
