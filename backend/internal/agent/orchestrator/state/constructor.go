package state

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

func NewState(req domain.ChatCommand, writer StreamWriter, opts InitOptions) *State {
	traceID := uuid.NewString()
	session := Session{
		SessionID:         req.SessionID,
		UserID:            req.UserID,
		RawQuery:          strings.TrimSpace(req.Message),
		PendingSelections: map[string]PendingSelection{},
		Intent:            domain.IntentUnknown,
		Route:             RouteUnknown,
		Slots:             map[string]any{},
		KBVersion:         opts.KBVersion,
		FeatureFlags:      opts.FeatureFlags,
		ReadOnly:          true,
		ResumeFromCP:      strings.TrimSpace(req.ResumeToken) != "",
	}
	return &State{
		StartedAt:    time.Now(),
		TraceID:      traceID,
		Request:      req,
		StreamWriter: writer,
		Recorder:     agenttool.NewSafeExecutionRecorder(),
		Session:      session,
		Response: &domain.ChatResult{
			SessionID:   req.SessionID,
			TraceID:     traceID,
			Status:      domain.ReplyStatusFallback,
			Intent:      domain.IntentUnknown,
			Trace:       domain.Trace{TraceID: traceID},
			Confidence:  0,
			NeedHandoff: false,
		},
	}
}

func (s *State) EnsureResponse() *domain.ChatResult {
	if s.Response == nil {
		s.Response = &domain.ChatResult{
			SessionID:   s.Request.SessionID,
			TraceID:     s.TraceID,
			Status:      domain.ReplyStatusFallback,
			Intent:      domain.IntentUnknown,
			Trace:       domain.Trace{TraceID: s.TraceID},
			Confidence:  0,
			NeedHandoff: false,
		}
	}
	if s.Response.Trace.TraceID == "" {
		s.Response.Trace.TraceID = s.TraceID
	}
	return s.Response
}

func (s *State) ToolExecutions() []domain.ToolExecution {
	if s == nil || s.Recorder == nil {
		return nil
	}
	return s.Recorder.Snapshot()
}
