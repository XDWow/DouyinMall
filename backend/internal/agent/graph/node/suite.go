package node

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/graph/observe"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/graph/prompt"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type Config struct {
	DefaultTenantID     string
	RateLimitPerMinute  int64
	ConversationWindow  int
	L0CacheTTL          time.Duration
	RerankTopK          int
	SummaryTriggerTurns int
	ToolParallelism     int
}

type Hooks struct {
	GenerateAnswer          func(context.Context, *orchestratorstate.FlowContext, []*schema.Message) (string, error)
	PersistConversationTurn func(context.Context, *orchestratorstate.FlowContext, string, dto.Intent, float64) error
	RegistryHasTool         func(context.Context, string) bool
	ApplyToolPlans          func(context.Context, *orchestratorstate.FlowContext, []dto.ToolCallPlan) (*orchestratorstate.FlowContext, error)
}

type Dependencies struct {
	Config          Config
	Model           model.ToolCallingChatModel
	Registry        *agenttool.Registry
	SessionStore    memory.Store
	Summarizer      memory.Summarizer
	ExactCache      cache.ExactCache
	RateLimiter     cache.RateLimiter
	CheckpointStore cache.CheckpointStore
	Prompts         *orchestratorprompt.Set
	Logger          logger.LoggerV1
	Metrics         *orchestratorobserve.Metrics
	Hooks           Hooks
}

type Suite struct {
	deps Dependencies
}

func NewSuite(deps Dependencies) *Suite {
	return &Suite{deps: deps}
}
