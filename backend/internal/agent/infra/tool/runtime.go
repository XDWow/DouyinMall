package tool

import (
	"context"
	"sync"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type recorderKey struct{}
type runtimeKey struct{}

// Runtime 保存一次工具执行链路需要透传的运行时信息。
// 这里不关心有哪些 tool，只关心这一次调用是谁、属于哪个 session、trace 是什么。
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

type SafeExecutionRecorder struct {
	mu    sync.Mutex
	items []domain.ToolExecution
}

func NewSafeExecutionRecorder() *SafeExecutionRecorder {
	return &SafeExecutionRecorder{}
}

func (r *SafeExecutionRecorder) Record(exec domain.ToolExecution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, exec)
}

func (r *SafeExecutionRecorder) Snapshot() []domain.ToolExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.ToolExecution, len(r.items))
	copy(out, r.items)
	return out
}
