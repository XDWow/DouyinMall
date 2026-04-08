package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/callback"
	orchestratorgraph "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/graph"
	aftersalenode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/aftersale"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	baseqanode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/fallback"
	inventorynode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/inventory"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	orchestratorragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// NewRuntime 根据配置和依赖组装完整的编排运行时。
func NewRuntime(ctx context.Context, cfg Config, deps Dependencies) (*Runtime, error) {
	cfg = applyConfigDefaults(cfg)

	log := deps.Logger
	if log == nil {
		log = logger.NewNopLogger()
	}
	metrics := deps.Metrics
	if metrics == nil {
		metrics = NewMetrics("agent")
	}
	prompts := deps.Prompts
	if prompts == nil {
		prompts = NewDefaultPrompts()
	}
	tracer := deps.Tracer
	if tracer == nil {
		tracer = otel.Tracer("douyinmall/agent")
	}

	svc := &Runtime{
		cfg:             cfg,
		model:           deps.Model,
		embedder:        deps.Embedder,
		knowledgeBase:   deps.KnowledgeBase,
		skills:          deps.Skills,
		registry:        deps.Registry,
		sessionService:  deps.SessionService,
		exactCache:      deps.ExactCache,
		semanticCache:   deps.SemanticCache,
		rateLimiter:     deps.RateLimiter,
		checkpointStore: deps.CheckpointStore,
		prompts:         prompts,
		logger:          log,
		metrics:         metrics,
		tracer:          tracer,
	}

	toolCheck := sharednode.ToolRegistryCheck(svc.registryHasTool)
	svc.callbackHandler = orchestratorcallback.Builder{Tracer: tracer, Metrics: metrics}.New()

	runner, err := (&orchestratorgraph.Builder{
		Config: orchestratorgraph.Config{
			InterruptBeforeNodes: cfg.InterruptBeforeNodes,
			InterruptAfterNodes:  cfg.InterruptAfterNodes,
		},
		CheckpointStore:   deps.CheckpointStore,
		Registry:          deps.Registry,
		AccessGuard:       globalnode.NewAccessGuardNode(cfg.DefaultTenantID, cfg.RateLimitPerMinute, deps.RateLimiter, deps.CheckpointStore),
		SessionLoad:       globalnode.NewSessionLoadNode(deps.SessionService),
		CachePreCheck:     globalnode.NewCachePreCheckNode(),
		L0ExactCache:      globalnode.NewL0ExactCacheNode(deps.ExactCache),
		L1SemanticCache:   globalnode.NewL1SemanticCacheNode(deps.SemanticCache, deps.Embedder, cfg.SemanticCacheScore, cfg.SemanticCacheTopK),
		QueryRewrite:      globalnode.NewQueryRewriteNode(deps.Model, prompts),
		IntentClassify:    globalnode.NewIntentClassifyNode(deps.Model, prompts),
		GlobalSlotExtract: globalnode.NewGlobalSlotExtractNode(),
		GlobalSlotCheck:   globalnode.NewGlobalSlotCheckNode(),
		AskUser:           globalnode.NewAskUserNode(),
		Route:             globalnode.NewRouteNode(),
		SkillSelect:       globalnode.NewSkillSelectNode(deps.Skills),
		Finalize: globalnode.NewFinalizeNode(
			deps.Model,
			prompts,
			deps.Skills,
			deps.Registry,
			deps.SessionService,
			globalnode.NewCacheWritebackService(deps.ExactCache, deps.SemanticCache, deps.Embedder, cfg.ExactCacheTTL, cfg.SemanticCacheTTL, log),
			log,
			metrics,
			cfg.MaxAnswerTokens,
		),
		OrderRead:           ordernode.NewOrderReadNode(toolCheck),
		InventoryRead:       inventorynode.NewInventoryReadNode(toolCheck),
		ProductInfo:         productnode.NewProductInfoNode(toolCheck),
		AddToCart:           cartnode.NewAddToCartNode(toolCheck),
		ReturnExchangeQuery: aftersalenode.NewReturnExchangeQueryNode(toolCheck),
		EligibilityCheck:    aftersalenode.NewEligibilityCheckNode(),
		ConfirmSummary:      aftersalenode.NewConfirmSummaryNode(),
		SubmitAfterSale:     aftersalenode.NewSubmitAfterSaleNode(),
		RAG:                 orchestratorragnode.NewRAGNode(deps.KnowledgeBase, cfg.RetrieveTopK, cfg.RetrieveMinScore),
		BaseQA:              baseqanode.NewBaseQANode(),
	}).Build(ctx)
	if err != nil {
		return nil, err
	}

	svc.runnable = runner
	return svc, nil
}

func (s *Runtime) Chat(ctx context.Context, req domain.ChatCommand) (*domain.ChatResult, error) {
	return s.run(ctx, req, nil)
}

func (s *Runtime) ChatStream(ctx context.Context, req domain.ChatCommand, writer StreamWriter) (*domain.ChatResult, error) {
	return s.run(ctx, req, writer)
}

func (s *Runtime) CreateSession(ctx context.Context, userID int64) (*domain.Session, error) {
	sessionID := "sess_" + uuid.NewString()
	now := time.Now()
	session := domain.Session{
		SessionID:  sessionID,
		UserID:     userID,
		Status:     domain.SessionStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		TotalTurns: 0,
	}
	if s.sessionService != nil {
		if err := s.sessionService.CreateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	return &session, nil
}

func (s *Runtime) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error) {
	if s.sessionService == nil {
		return nil, 0, nil
	}
	messages, err := s.sessionService.AllMessages(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	total := len(messages)
	if total == 0 {
		return nil, 0, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []domain.Message{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return append([]domain.Message(nil), messages[offset:end]...), total, nil
}

func (s *Runtime) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	if s.sessionService == nil {
		return nil, 0, nil
	}
	return s.sessionService.ListSessions(ctx, userID, limit, offset)
}

func (s *Runtime) ClearSession(ctx context.Context, sessionID string) error {
	if s.sessionService == nil {
		return nil
	}
	return s.sessionService.Clear(ctx, sessionID)
}

func (s *Runtime) run(ctx context.Context, req domain.ChatCommand, writer StreamWriter) (*domain.ChatResult, error) {
	start := time.Now()
	state := orchestratorstate.NewState(req, writer, orchestratorstate.InitOptions{
		KBVersion:    cfgValue(s.cfg.KBVersion, DefaultConfig().KBVersion),
		FeatureFlags: s.cfg.FeatureFlags,
	})

	checkpointID := req.ResumeToken
	if checkpointID == "" {
		checkpointID = state.TraceID
	}
	state.Checkpoint = checkpointID
	if strings.TrimSpace(state.Request.SessionID) == "" {
		state.Request.SessionID = "sess_" + state.TraceID
	}

	ctx = agenttool.WithExecutionRecorder(ctx, state.Recorder)
	ctx = agenttool.WithRuntime(ctx, agenttool.Runtime{
		UserID:    state.Request.UserID,
		SessionID: state.Request.SessionID,
		TraceID:   state.TraceID,
	})

	orchestratorobserve.SendEvent(ctx, writer, "start", map[string]any{
		"trace_id":      state.TraceID,
		"checkpoint_id": checkpointID,
		"session_id":    state.Request.SessionID,
	})

	invokeOpts := make([]compose.Option, 0, 2)
	if s.checkpointStore != nil {
		invokeOpts = append(invokeOpts,
			compose.WithCheckPointID(checkpointID),
			compose.WithWriteToCheckPointID(checkpointID),
		)
	}
	if s.callbackHandler != nil {
		invokeOpts = append(invokeOpts, compose.WithCallbacks(s.callbackHandler))
	}

	out, err := s.runnable.Invoke(ctx, map[string]any{"flow": state}, invokeOpts...)
	if out == nil {
		out = state
	}

	resp := out.EnsureResponse()
	resp.Trace.TraceID = out.TraceID
	resp.Trace.CheckpointID = checkpointID

	if info, ok := compose.ExtractInterruptInfo(err); ok {
		resp.Status = domain.ReplyStatusFallback
		resp.Interrupt = &domain.InterruptInfo{CheckpointID: checkpointID}
		if info != nil {
			resp.Interrupt.RerunNodes = append(resp.Interrupt.RerunNodes, info.RerunNodes...)
		}
		if strings.TrimSpace(resp.Reply) == "" {
			resp.Reply = "\u8bf7\u5148\u8865\u5145\u6240\u9700\u4fe1\u606f\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u3002"
		}
		s.metrics.ObserveRequest(string(resp.Status), time.Since(start))
		orchestratorobserve.SendEvent(ctx, writer, "done", resp)
		return resp, nil
	}

	if err != nil {
		orchestratorobserve.SendEvent(ctx, writer, "error", map[string]any{"message": err.Error()})
		s.metrics.ObserveRequest("error", time.Since(start))
		return nil, err
	}

	s.metrics.ObserveRequest(string(resp.Status), time.Since(start))
	orchestratorobserve.SendEvent(ctx, writer, "done", resp)
	return resp, nil
}

func (s *Runtime) registryHasTool(ctx context.Context, name string) bool {
	if s.registry == nil {
		return false
	}
	_ = ctx
	return s.registry.Has(name)
}

func cfgValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
