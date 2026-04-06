package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	agentmemory "github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/callback"
	orchestratorgraph "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/graph"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/rag"
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
		memory:          deps.Memory,
		exactCache:      deps.ExactCache,
		semanticCache:   deps.SemanticCache,
		rateLimiter:     deps.RateLimiter,
		checkpointStore: deps.CheckpointStore,
		prompts:         prompts,
		logger:          log,
		metrics:         metrics,
		tracer:          tracer,
	}

	toolCheck := orchestratornode.ToolRegistryCheck(svc.registryHasTool)
	persistTurn := orchestratornode.ConversationTurnPersister(svc.persistConversationTurn)
	genAnswer := orchestratornode.AnswerGenerator(svc.generateAnswer)
	svc.callbackHandler = orchestratorcallback.Builder{Tracer: tracer, Metrics: metrics}.New()

	runner, err := (&orchestratorgraph.Builder{
		Config: orchestratorgraph.Config{
			InterruptBeforeNodes: cfg.InterruptBeforeNodes,
			InterruptAfterNodes:  cfg.InterruptAfterNodes,
		},
		CheckpointStore:     deps.CheckpointStore,
		Registry:            deps.Registry,
		AccessGuard:         orchestratornode.NewAccessGuardNode(cfg.DefaultTenantID, cfg.RateLimitPerMinute, deps.RateLimiter, deps.CheckpointStore),
		SessionLoad:         orchestratornode.NewSessionLoadNode(),
		CachePolicy:         orchestratornode.NewCachePolicyNode(),
		MultiLevelCache:     orchestratornode.NewMultiLevelCacheNode(deps.ExactCache, deps.SemanticCache, deps.Embedder, cfg.SemanticCacheScore, cfg.SemanticCacheTopK),
		IntentClassify:      orchestratornode.NewIntentClassifyNode(deps.Model, prompts),
		SlotExtract:         orchestratornode.NewSlotExtractNode(),
		SlotCheck:           orchestratornode.NewSlotCheckNode(),
		AskUser:             orchestratornode.NewAskUserNode(persistTurn, log),
		Route:               orchestratornode.NewRouteNode(),
		SkillSelect:         orchestratornode.NewSkillSelectNode(deps.Skills),
		ResponseRender:      orchestratornode.NewResponseRenderNode(deps.Model, prompts, deps.Skills, genAnswer, metrics),
		CacheWriteback:      orchestratornode.NewCacheWritebackNode(deps.ExactCache, deps.SemanticCache, deps.Embedder, cfg.ExactCacheTTL, cfg.SemanticCacheTTL, persistTurn, log),
		OrderRead:           orchestratornode.NewOrderReadNode(toolCheck),
		InventoryRead:       orchestratornode.NewInventoryReadNode(toolCheck),
		ProductInfo:         orchestratornode.NewProductInfoNode(toolCheck),
		AddToCart:           orchestratornode.NewAddToCartNode(toolCheck),
		ReturnExchangeQuery: orchestratornode.NewReturnExchangeQueryNode(toolCheck),
		EligibilityCheck:    orchestratornode.NewEligibilityCheckNode(),
		ConfirmSummary:      orchestratornode.NewConfirmSummaryNode(persistTurn, log),
		SubmitAfterSale:     orchestratornode.NewSubmitAfterSaleNode(),
		RAG:                 orchestratorragnode.NewRAGNode(deps.KnowledgeBase, cfg.RetrieveTopK, cfg.RetrieveMinScore),
		Fallback:            orchestratornode.NewFallbackNode(),
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
	if s.memory != nil {
		if err := s.memory.CreateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	return &session, nil
}

func (s *Runtime) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error) {
	if s.memory == nil {
		return nil, 0, nil
	}
	messages, err := s.memory.AllMessages(ctx, sessionID)
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
	if s.memory == nil {
		return nil, 0, nil
	}
	return s.memory.ListSessions(ctx, userID, limit, offset)
}

func (s *Runtime) ClearSession(ctx context.Context, sessionID string) error {
	if s.memory == nil {
		return nil
	}
	return s.memory.Clear(ctx, sessionID)
}

func (s *Runtime) run(ctx context.Context, req domain.ChatCommand, writer StreamWriter) (*domain.ChatResult, error) {
	start := time.Now()
	state := orchestratorstate.NewConversationState(req, writer, orchestratorstate.InitOptions{
		KBVersion:    cfgValue(s.cfg.KBVersion, DefaultConfig().KBVersion),
		FeatureFlags: s.cfg.FeatureFlags,
	})

	checkpointID := req.ResumeToken
	if checkpointID == "" {
		checkpointID = state.TraceID
	}
	state.Checkpoint = checkpointID

	if err := s.loadSessionContext(ctx, state); err != nil {
		orchestratorobserve.SendEvent(ctx, writer, "error", map[string]any{"message": err.Error()})
		s.metrics.ObserveRequest("error", time.Since(start))
		return nil, err
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
			resp.Reply = "请先补充所需信息，我再继续为你处理。"
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

func (s *Runtime) generateAnswer(ctx context.Context, state *orchestratorstate.ConversationState, messages []*schema.Message) (string, error) {
	if s.model == nil {
		return "", fmt.Errorf("chat model is not configured")
	}

	if state.StreamWriter == nil {
		msg, err := s.model.Generate(ctx, messages,
			model.WithTemperature(0.15),
			model.WithMaxTokens(s.cfg.MaxAnswerTokens),
			model.WithToolChoice(schema.ToolChoiceForbidden),
		)
		if err != nil || msg == nil {
			return "", err
		}
		return msg.Content, nil
	}

	stream, err := s.model.Stream(ctx, messages,
		model.WithTemperature(0.15),
		model.WithMaxTokens(s.cfg.MaxAnswerTokens),
		model.WithToolChoice(schema.ToolChoiceForbidden),
	)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var builder strings.Builder
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return "", recvErr
		}
		if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		builder.WriteString(chunk.Content)
		orchestratorobserve.SendEvent(ctx, state.StreamWriter, "token", map[string]any{
			"trace_id": state.TraceID,
			"text":     chunk.Content,
		})
	}
	return builder.String(), nil
}

func (s *Runtime) registryHasTool(ctx context.Context, name string) bool {
	if s.registry == nil {
		return false
	}
	for _, tool := range s.registry.Tools() {
		info, err := tool.Info(ctx)
		if err == nil && info != nil && info.Name == name {
			return true
		}
	}
	return false
}

func (s *Runtime) loadSessionContext(ctx context.Context, state *orchestratorstate.ConversationState) error {
	if state == nil {
		return fmt.Errorf("state is required")
	}

	sessionID := strings.TrimSpace(state.Request.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + state.TraceID
		state.Request.SessionID = sessionID
	}

	if s.memory == nil {
		now := time.Now()
		state.SessionMeta = &domain.Session{
			SessionID:  sessionID,
			UserID:     state.Request.UserID,
			Status:     domain.SessionStatusActive,
			CreatedAt:  now,
			UpdatedAt:  now,
			TotalTurns: 0,
		}
		state.Session.SessionID = sessionID
		state.EnsureResponse().SessionID = sessionID
		return nil
	}

	meta, messages, err := s.memory.LoadSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if meta == nil {
		now := time.Now()
		meta = &domain.Session{
			SessionID:  sessionID,
			UserID:     state.Request.UserID,
			Status:     domain.SessionStatusActive,
			CreatedAt:  now,
			UpdatedAt:  now,
			TotalTurns: 0,
		}
		if err := s.memory.CreateSession(ctx, *meta); err != nil {
			return err
		}
	}
	if meta.UserID != 0 && meta.UserID != state.Request.UserID {
		return fmt.Errorf("session owner mismatch")
	}

	cloned := *meta
	state.SessionMeta = &cloned
	orchestratorstate.SetRecentMessages(state, s.memory.RecentSchemaMessages(messages))
	state.Session.SessionID = meta.SessionID
	state.EnsureResponse().SessionID = meta.SessionID
	return nil
}

func (s *Runtime) persistConversationTurn(
	ctx context.Context,
	state *orchestratorstate.ConversationState,
	assistantReply string,
	assistantIntent domain.Intent,
	confidence float64,
) error {
	if state == nil || state.SessionMeta == nil || s.memory == nil {
		return nil
	}

	userMsg := agentmemory.NewUserMessage(state.SessionMeta.SessionID, state.Session.RawQuery)
	assistantMsg := agentmemory.NewAssistantMessage(state.SessionMeta.SessionID, assistantReply, assistantIntent, confidence)

	appendMessages := make([]domain.Message, 0, 2)
	if strings.TrimSpace(userMsg.Content) != "" {
		appendMessages = append(appendMessages, userMsg)
	}
	if strings.TrimSpace(assistantMsg.Content) != "" {
		appendMessages = append(appendMessages, assistantMsg)
	}
	if len(appendMessages) == 0 {
		return nil
	}

	cloned := *state.SessionMeta
	cloned.TotalTurns++
	cloned.UpdatedAt = assistantMsg.CreatedAt
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = time.Now()
	}
	if assistantMsg.Content != "" {
		cloned.LastMessage = agentmemory.Truncate(assistantMsg.Content, 64)
	} else {
		cloned.LastMessage = agentmemory.Truncate(userMsg.Content, 64)
	}
	if cloned.Status == "" {
		cloned.Status = domain.SessionStatusActive
	}

	if err := s.memory.SaveTurn(ctx, cloned, userMsg, assistantMsg); err != nil {
		return err
	}
	state.SessionMeta = &cloned
	return nil
}

func cfgValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
