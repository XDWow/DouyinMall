package node

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type Config struct {
	DefaultTenantID    string
	RateLimitPerMinute int64
	ConversationWindow int
	L0CacheTTL         time.Duration
	RerankTopK         int
	ToolParallelism    int
}

type Hooks struct {
	GenerateAnswer          func(context.Context, *orchestratorstate.ConversationState, []*schema.Message) (string, error)
	PersistConversationTurn func(context.Context, *orchestratorstate.ConversationState, string, domain.Intent, float64) error
	RegistryHasTool         func(context.Context, string) bool
	ApplyToolPlans          func(context.Context, *orchestratorstate.ConversationState, []domain.ToolCallPlan) (*orchestratorstate.ConversationState, error)
}

type Dependencies struct {
	Config          Config
	Model           model.ToolCallingChatModel
	Registry        *agenttool.Registry
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

