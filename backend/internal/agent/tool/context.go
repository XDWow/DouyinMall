package tool

import "context"

type recorderKey struct{}
type runtimeKey struct{}

type Runtime struct {
	UserID    int64
	SessionID string
	TraceID   string
}

func WithExecutionRecorder(ctx context.Context, recorder ExecutionRecorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, recorder)
}

func WithRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}

func executionRecorderFromContext(ctx context.Context) ExecutionRecorder {
	recorder, _ := ctx.Value(recorderKey{}).(ExecutionRecorder)
	return recorder
}

func runtimeFromContext(ctx context.Context) Runtime {
	runtime, _ := ctx.Value(runtimeKey{}).(Runtime)
	return runtime
}
