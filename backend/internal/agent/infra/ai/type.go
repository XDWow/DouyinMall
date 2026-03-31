package ai

import (
	"context"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
)

// 这里是为业务实现 ai chat，调用底层 pkg/ai，承上启下
// 可以做的事情是：面向业务，给业务暴露简单的接口，然后自己补充一些固定信息，比如模型选择，配置，业务方不用关心这个
// 还有，流式的时候的tool calling 的参数收集，收满了再传给业务方调用 mcp server
// 以及服务治理，熔断，限流，降级
type CSLLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error)
}

// 业务方要传什么？简化接口，隐藏底层细节
// Model、Temperature、MaxTokens 等参数由 infra 层统一配置
type ChatRequest struct {
	Messages []pkgai.Message `json:"messages"`
	Tools    []pkgai.ToolDef `json:"tools,omitempty"`
}

// 业务方需要拿到什么？
type ChatResponse struct {
	ID         string         `json:"id"`
	Created    int64          `json:"created"`
	Choices    []pkgai.Choice `json:"choices"`
	TokensUsed int            `json:"tokens_used,omitempty"` // 用于记录和监控
}
