package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/callback"
	orchestratorgraph "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/graph"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	understandingnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global/understanding"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

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
	tracer := deps.Tracer
	if tracer == nil {
		tracer = otel.Tracer("douyinmall/agent")
	}

	svc := &Runtime{
		cfg:             cfg,
		llms:            deps.LLMs,
		embedder:        deps.Embedder,
		knowledgeBase:   deps.KnowledgeBase,
		skills:          deps.Skills,
		registry:        deps.Registry,
		sessionService:  deps.SessionService,
		exactCache:      deps.ExactCache,
		semanticCache:   deps.SemanticCache,
		rateLimiter:     deps.RateLimiter,
		checkpointStore: deps.CheckpointStore,
		logger:          log,
		metrics:         metrics,
		tracer:          tracer,
	}

	svc.callbackHandler = orchestratorcallback.Builder{Tracer: tracer, Metrics: metrics}.New()

	runner, err := (&orchestratorgraph.Builder{
		Config: orchestratorgraph.Config{
			InterruptBeforeNodes: cfg.InterruptBeforeNodes,
		},
		CheckpointStore: deps.CheckpointStore,
		Registry:        deps.Registry,
		Skills:          deps.Skills,
		AgentModel:      deps.LLMs.Strong(),
		AccessGuard:     globalnode.NewAccessGuardNode(cfg.DefaultTenantID, cfg.RateLimitPerMinute, deps.RateLimiter),
		SessionLoad:     globalnode.NewSessionLoadNode(deps.SessionService),
		Understanding:   understandingnode.NewUnderstandingNode(deps.LLMs.Weak()),
		CacheLookup:     globalnode.NewCacheLookupNode(deps.ExactCache, deps.SemanticCache, deps.Embedder, cfg.SemanticCacheScore, cfg.SemanticCacheTopK, cfg.DefaultTenantID),
		Route:           globalnode.NewRouteNode(),
		Finalize: globalnode.NewFinalizeNode(
			deps.SessionService,
			globalnode.NewCacheWritebackService(deps.ExactCache, deps.SemanticCache, deps.Embedder, cfg.ExactCacheTTL, cfg.SemanticCacheTTL, log),
			log,
			metrics,
		),
		RAG: ragnode.NewRAGNode(deps.KnowledgeBase, cfg.RetrieveTopK, cfg.RetrieveMinScore),
	}).Build(ctx)
	if err != nil {
		return nil, err
	}

	svc.runnable = runner
	return svc, nil
}

func (s *Runtime) Chat(ctx context.Context, req *domain.ChatInput) (*domain.ChatResult, error) {
	return s.run(ctx, req, nil)
}

func (s *Runtime) ChatStream(ctx context.Context, req *domain.ChatInput, writer StreamWriter) (*domain.ChatResult, error) {
	return s.run(ctx, req, writer)
}

func (s *Runtime) Resume(ctx context.Context, in domain.WorkflowResumeInput) (*domain.ChatResult, error) {
	if strings.TrimSpace(in.CheckpointID) == "" || strings.TrimSpace(in.InterruptID) == "" {
		return nil, fmt.Errorf("checkpoint_id and interrupt_id are required")
	}
	if in.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	return s.run(ctx, &domain.ChatInput{
		SessionID:   strings.TrimSpace(in.SessionID),
		UserID:      in.UserID,
		ResumeToken: strings.TrimSpace(in.CheckpointID),
		InterruptID: strings.TrimSpace(in.InterruptID),
		ResumeData:  domain.CloneAnyMap(in.ResumeData),
	}, nil)
}

func (s *Runtime) CreateSession(ctx context.Context, userID int64) (*domain.SessionListItem, error) {
	sessionID := "sess_" + uuid.NewString()
	now := time.Now()
	user := domain.Session{
		SessionID: sessionID,
		UserID:    userID,
	}
	meta := domain.SessionTableMeta{
		Status:     domain.SessionStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		TotalTurns: 0,
	}
	if s.sessionService != nil {
		if err := s.sessionService.CreateSession(ctx, user, meta); err != nil {
			return nil, err
		}
	}
	return &domain.SessionListItem{Context: user, Meta: meta}, nil
}

func (s *Runtime) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.SessionMessage, int, error) {
	if s.sessionService == nil {
		return nil, 0, nil
	}
	messages, err := s.sessionService.AllMessages(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	total := len(messages)
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []domain.SessionMessage{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return append([]domain.SessionMessage(nil), messages[offset:end]...), total, nil
}

func (s *Runtime) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.SessionListItem, int, error) {
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

func (s *Runtime) run(ctx context.Context, req *domain.ChatInput, writer StreamWriter) (*domain.ChatResult, error) {
	start := time.Now()
	if req == nil {
		req = &domain.ChatInput{}
	}
	state := domain.NewState(req)
	rec := agenttool.NewSafeExecutionRecorder()

	checkpointID := strings.TrimSpace(req.ResumeToken)
	if checkpointID == "" {
		checkpointID = state.TraceID
	}
	state.EnsureResponse().Trace.CheckpointID = checkpointID
	if strings.TrimSpace(state.Session.SessionID) == "" {
		state.Session.SessionID = "sess_" + state.TraceID
	}

	if checkpointID != "" && req.ResumeToken != "" && s.checkpointStore != nil {
		_, ok, err := s.checkpointStore.Get(ctx, checkpointID)
		if err != nil {
			return nil, fmt.Errorf("checkpoint get: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("checkpoint not found")
		}
	}
	if id := strings.TrimSpace(req.InterruptID); id != "" {
		if len(req.ResumeData) > 0 {
			ctx = compose.ResumeWithData(ctx, id, req.ResumeData)
		} else {
			ctx = compose.Resume(ctx, id)
		}
	}

	ctx = domain.WithInitialState(ctx, state)
	ctx = agenttool.WithExecutionRecorder(ctx, rec)
	ctx = agenttool.WithStreamWriter(ctx, writer)
	ctx = agenttool.WithRuntime(ctx, agenttool.Runtime{
		UserID:    req.UserID,
		SessionID: state.Session.SessionID,
		TraceID:   state.TraceID,
	})

	sw := agenttool.StreamWriterFrom(ctx)
	orchestratorobserve.SendEvent(ctx, sw, "start", map[string]any{
		"trace_id":      state.TraceID,
		"checkpoint_id": checkpointID,
		"session_id":    state.Session.SessionID,
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

	out, err := s.runnable.Invoke(ctx, struct{}{}, invokeOpts...)
	if out == nil {
		out = state
	}

	resp := out.EnsureResponse()
	resp.Trace.TraceID = out.TraceID
	resp.Trace.CheckpointID = checkpointID

	if info, ok := compose.ExtractInterruptInfo(err); ok {
		resp.Status = domain.ReplyStatusFallback
		detail := extractInterruptDetailMap(info)
		resp.Interrupt = &domain.InterruptInfo{
			CheckpointID: checkpointID,
			InterruptID:  interruptCtxIDFromCompose(info),
			Detail:       detail,
		}
		resp.Interrupted = true
		if info != nil {
			resp.Interrupt.RerunNodes = append(resp.Interrupt.RerunNodes, info.RerunNodes...)
		}
		if strings.TrimSpace(resp.Reply) == "" {
			if question, ok := detail["question"].(string); ok && strings.TrimSpace(question) != "" {
				resp.Reply = strings.TrimSpace(question)
			} else {
				resp.Reply = "请补充缺失信息。"
			}
		}
		s.recordRequestLatencyInsight(resp, time.Since(start))
		s.metrics.ObserveRequest(string(resp.Status), time.Since(start))
		orchestratorobserve.SendEvent(ctx, sw, "done", resp)
		return resp, nil
	}
	if err != nil {
		orchestratorobserve.SendEvent(ctx, sw, "error", map[string]any{"message": err.Error()})
		s.metrics.ObserveRequest("error", time.Since(start))
		return nil, err
	}

	s.recordRequestLatencyInsight(resp, time.Since(start))
	s.metrics.ObserveRequest(string(resp.Status), time.Since(start))
	orchestratorobserve.SendEvent(ctx, sw, "done", resp)
	return resp, nil
}

func (s *Runtime) recordRequestLatencyInsight(resp *domain.ChatResult, wall time.Duration) {
	if resp == nil {
		return
	}
	orchestratorobserve.EnrichTraceSlowest(resp)
	s.metrics.ObserveRequestBottleneck(resp.Trace.SlowestStepNode, resp.Trace.SlowestStepLatencyMs)
	if resp.Trace.SlowestStepNode == "" {
		return
	}
	s.logger.Debug("chat_latency_breakdown",
		logger.String("trace_id", resp.TraceID),
		logger.String("slowest_node", resp.Trace.SlowestStepNode),
		logger.Int64("slowest_step_ms", resp.Trace.SlowestStepLatencyMs),
		logger.Int64("wall_total_ms", wall.Milliseconds()),
		logger.Int("workflow_step_count", len(resp.Trace.Steps)),
	)
}

func extractInterruptDetailMap(info *compose.InterruptInfo) map[string]any {
	if info == nil {
		return nil
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil && ic.IsRootCause {
			if detail, ok := ic.Info.(map[string]any); ok {
				return detail
			}
		}
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil {
			if detail, ok := ic.Info.(map[string]any); ok {
				return detail
			}
		}
	}
	return nil
}

func interruptCtxIDFromCompose(info *compose.InterruptInfo) string {
	if info == nil {
		return ""
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil && ic.IsRootCause && ic.ID != "" {
			return ic.ID
		}
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil && ic.ID != "" {
			return ic.ID
		}
	}
	return ""
}
