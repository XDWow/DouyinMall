package domain

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

func NewState(req ChatCommand, writer StreamWriter, recorder ToolExecutionSink) *State {
	traceID := uuid.NewString()
	session := Session{
		SessionID:         req.SessionID,
		UserID:            req.UserID,
		Status:            SessionStatusActive,
		Slots:             map[string]any{},
		TotalTurns:        0,
		RawQuery:          strings.TrimSpace(req.Message),
		PendingSelections: map[string]PendingSelection{},
		Intent:            IntentUnknown,
		Route:             RouteUnknown,
		ReadOnly:          true,
		ResumeFromCP:      strings.TrimSpace(req.ResumeToken) != "",
	}
	return &State{
		Input: TurnInput(req),
		StartedAt:    time.Now(),
		TraceID:      traceID,
		StreamWriter: writer,
		Recorder:     recorder,
		Session:      session,
		Response: &ChatResult{
			SessionID:   req.SessionID,
			TraceID:     traceID,
			Status:      ReplyStatusFallback,
			Intent:      IntentUnknown,
			Trace:       Trace{TraceID: traceID},
			Confidence:  0,
			NeedHandoff: false,
		},
	}
}

func ProcessState(ctx context.Context, handler func(*State) error) error {
	return compose.ProcessState(ctx, func(_ context.Context, s *State) error {
		if s == nil {
			return nil
		}
		return handler(s)
	})
}
