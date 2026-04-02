package graph

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

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorgraph "github.com/XDWow/DouyinMall/backend/internal/agent/graph/app"
	orchestratorcallback "github.com/XDWow/DouyinMall/backend/internal/agent/graph/callback"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/graph/node"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/graph/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	orchestratorworkflow "github.com/XDWow/DouyinMall/backend/internal/agent/graph/workflow"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
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
	prompts := deps.Prompts
	if prompts == nil {
		prompts = NewDefaultPrompts()
	}
	summarizer := deps.Summarizer
	if summarizer == nil {
		summarizer = memory.NewFallbackSummarizer()
	}
	tracer := deps.Tracer
	if tracer == nil {
		tracer = otel.Tracer("douyinmall/agent")
	}

	svc := &Runtime{
		cfg:             cfg,
		model:           deps.Model,
		embedder:        deps.Embedder,
		retriever:       deps.Retriever,
		registry:        deps.Registry,
		sessionStore:    deps.SessionStore,
		summarizer:      summarizer,
		exactCache:      deps.ExactCache,
		rateLimiter:     deps.RateLimiter,
		checkpointStore: deps.CheckpointStore,
		prompts:         prompts,
		logger:          log,
		metrics:         metrics,
		tracer:          tracer,
	}

	nodes := orchestratornode.NewSuite(orchestratornode.Dependencies{
		Config: orchestratornode.Config{
			DefaultTenantID:     cfg.DefaultTenantID,
			RateLimitPerMinute:  cfg.RateLimitPerMinute,
			ConversationWindow:  cfg.ConversationWindow,
			L0CacheTTL:          cfg.L0CacheTTL,
			RerankTopK:          cfg.RerankTopK,
			SummaryTriggerTurns: cfg.SummaryTriggerTurns,
			ToolParallelism:     cfg.ToolParallelism,
		},
		Model:           deps.Model,
		Registry:        deps.Registry,
		SessionStore:    deps.SessionStore,
		Summarizer:      summarizer,
		ExactCache:      deps.ExactCache,
		RateLimiter:     deps.RateLimiter,
		CheckpointStore: deps.CheckpointStore,
		Prompts:         prompts,
		Logger:          log,
		Metrics:         metrics,
		Hooks: orchestratornode.Hooks{
			GenerateAnswer:          svc.generateAnswer,
			PersistConversationTurn: svc.persistConversationTurn,
			RegistryHasTool:         svc.registryHasTool,
			ApplyToolPlans:          svc.applyToolPlans,
		},
	})
	workflows := &orchestratorworkflow.Builder{
		Model:     deps.Model,
		Retriever: deps.Retriever,
		Registry:  deps.Registry,
		Prompts:   prompts,
		Nodes:     nodes,
	}
	svc.callbackHandler = orchestratorcallback.Builder{Tracer: tracer, Metrics: metrics}.New()

	runner, err := (&orchestratorgraph.Builder{
		Config: orchestratorgraph.Config{
			InterruptBeforeNodes: cfg.InterruptBeforeNodes,
			InterruptAfterNodes:  cfg.InterruptAfterNodes,
		},
		CheckpointStore: deps.CheckpointStore,
		Nodes:           nodes,
		Workflows:       workflows,
	}).Build(ctx)
	if err != nil {
		return nil, err
	}
	svc.runnable = runner
	return svc, nil
}

func (s *Runtime) Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, error) {
	return s.run(ctx, req, nil)
}

func (s *Runtime) ChatStream(ctx context.Context, req dto.ChatRequest, writer StreamWriter) (*dto.ChatResponse, error) {
	return s.run(ctx, req, writer)
}

func (s *Runtime) CreateSession(ctx context.Context, userID int64, channel string) (*dto.Session, error) {
	session := &memory.Session{
		ID:        "sess_" + uuid.NewString(),
		UserID:    userID,
		Channel:   channel,
		Status:    dto.SessionStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if s.sessionStore != nil {
		if err := s.sessionStore.Create(ctx, session); err != nil {
			return nil, err
		}
	}
	return &dto.Session{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Channel:     session.Channel,
		Status:      session.Status,
		Summary:     session.Summary,
		LastMessage: session.LastMessagePreview(),
		TotalTurns:  session.TotalTurns,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}, nil
}

func (s *Runtime) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]dto.Message, int, error) {
	if s.sessionStore == nil {
		return nil, 0, nil
	}
	session, err := s.sessionStore.Load(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	total := len(session.Messages)
	if limit <= 0 {
		limit = 20
	}
	if offset >= total {
		return []dto.Message{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return append([]dto.Message(nil), session.Messages[offset:end]...), total, nil
}

func (s *Runtime) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]dto.Session, int, error) {
	if s.sessionStore == nil {
		return nil, 0, nil
	}
	sessions, total, err := s.sessionStore.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.Session, 0, len(sessions))
	for i := range sessions {
		session := sessions[i]
		result = append(result, dto.Session{
			SessionID:   session.ID,
			UserID:      session.UserID,
			Channel:     session.Channel,
			Status:      session.Status,
			Summary:     session.Summary,
			LastMessage: session.LastMessagePreview(),
			TotalTurns:  session.TotalTurns,
			CreatedAt:   session.CreatedAt,
			UpdatedAt:   session.UpdatedAt,
		})
	}
	return result, total, nil
}

func (s *Runtime) ClearSession(ctx context.Context, sessionID string) error {
	if s.sessionStore == nil {
		return nil
	}
	return s.sessionStore.Clear(ctx, sessionID)
}

func (s *Runtime) run(ctx context.Context, req dto.ChatRequest, writer StreamWriter) (*dto.ChatResponse, error) {
	start := time.Now()
	flow := orchestratorstate.NewFlowContext(req, writer, orchestratorstate.InitOptions{
		KBVersion:    cfgValue(s.cfg.KBVersion, DefaultConfig().KBVersion),
		FeatureFlags: s.cfg.FeatureFlags,
	})
	checkpointID := req.ResumeToken
	if checkpointID == "" {
		checkpointID = flow.TraceID
	}
	flow.Checkpoint = checkpointID
	ctx = agenttool.WithExecutionRecorder(ctx, flow.Recorder)
	ctx = agenttool.WithRuntime(ctx, agenttool.Runtime{
		UserID:    flow.Request.UserID,
		SessionID: flow.Request.SessionID,
		TraceID:   flow.TraceID,
	})

	orchestratorobserve.SendEvent(ctx, writer, "start", map[string]any{
		"trace_id":      flow.TraceID,
		"checkpoint_id": checkpointID,
		"session_id":    req.SessionID,
	})

	invokeOpts := make([]compose.Option, 0, 2)
	if s.checkpointStore != nil {
		invokeOpts = append(invokeOpts, compose.WithCheckPointID(checkpointID), compose.WithWriteToCheckPointID(checkpointID))
	}
	if s.callbackHandler != nil {
		invokeOpts = append(invokeOpts, compose.WithCallbacks(s.callbackHandler))
	}

	out, err := s.runnable.Invoke(ctx, map[string]any{"flow": flow}, invokeOpts...)
	if out == nil {
		out = flow
	}
	resp := out.EnsureResponse()
	resp.Trace.TraceID = out.TraceID
	resp.Trace.CheckpointID = checkpointID

	if info, ok := compose.ExtractInterruptInfo(err); ok {
		resp.Status = dto.ReplyStatusFallback
		resp.Interrupt = &dto.InterruptInfo{CheckpointID: checkpointID}
		if info != nil {
			resp.Interrupt.RerunNodes = append(resp.Interrupt.RerunNodes, info.RerunNodes...)
		}
		if strings.TrimSpace(resp.Reply) == "" {
			resp.Reply = "Please provide the required information so the workflow can continue."
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

func (s *Runtime) generateAnswer(ctx context.Context, flow *orchestratorstate.FlowContext, messages []*schema.Message) (string, error) {
	if s.model == nil {
		return "", fmt.Errorf("chat model is not configured")
	}
	if flow.StreamWriter == nil {
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
		orchestratorobserve.SendEvent(ctx, flow.StreamWriter, "token", map[string]any{
			"trace_id": flow.TraceID,
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

func (s *Runtime) applyToolPlans(ctx context.Context, flow *orchestratorstate.FlowContext, plans []dto.ToolCallPlan) (*orchestratorstate.FlowContext, error) {
	message, err := orchestratorworkflow.CreateToolDecisionMessage(plans)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i].RawJSON = message.ToolCalls[i].Function.Arguments
	}
	flow.Tool.Plans = plans
	flow.Tool.DecisionMessage = message
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (s *Runtime) persistConversationTurn(ctx context.Context, flow *orchestratorstate.FlowContext, assistantReply string, assistantIntent dto.Intent, confidence float64) error {
	if flow == nil || flow.Session == nil || s.sessionStore == nil {
		return nil
	}
	now := time.Now()
	flow.Session.Messages = append(flow.Session.Messages, dto.Message{
		ID:        uuid.NewString(),
		SessionID: flow.Session.ID,
		Role:      dto.RoleUser,
		Content:   flow.State.RawQuery,
		CreatedAt: now,
	})
	if strings.TrimSpace(assistantReply) != "" {
		flow.Session.Messages = append(flow.Session.Messages, dto.Message{
			ID:         uuid.NewString(),
			SessionID:  flow.Session.ID,
			Role:       dto.RoleAssistant,
			Content:    assistantReply,
			Intent:     assistantIntent,
			Confidence: confidence,
			CreatedAt:  now,
		})
		flow.Session.TotalTurns++
	}
	flow.Session.UpdatedAt = now
	if flow.Session.TotalTurns >= s.cfg.SummaryTriggerTurns && s.summarizer != nil {
		if summary, err := s.summarizer.Summarize(ctx, flow.Session); err == nil && summary != "" {
			flow.Session.Summary = summary
		}
	}
	if err := s.sessionStore.Save(ctx, flow.Session); err == nil {
		return nil
	}
	return s.sessionStore.Create(ctx, flow.Session)
}

func cfgValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
