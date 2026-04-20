package domain

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

type initialStateKey struct{}

func NewState(req *ChatInput) *State {
	if req == nil {
		req = &ChatInput{}
	}
	traceID := uuid.NewString()
	sess := &Session{
		SessionID: strings.TrimSpace(req.SessionID),
		UserID:    req.UserID,
	}
	return &State{
		Input:          req,
		TraceID:        traceID,
		Session:        sess,
		Intent:         IntentUnknown,
		RewrittenQuery: "",
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

func WithInitialState(ctx context.Context, st *State) context.Context {
	return context.WithValue(ctx, initialStateKey{}, st)
}

// SharedGraphState 让主图和子图共用同一份 State 指针。
func SharedGraphState(ctx context.Context) *State {
	if st, ok := ctx.Value(initialStateKey{}).(*State); ok && st != nil {
		return st
	}

	var shared *State
	_ = compose.ProcessState[*State](ctx, func(_ context.Context, st *State) error {
		shared = st
		return nil
	})
	if shared != nil {
		return shared
	}
	return &State{}
}

func ProcessState(ctx context.Context, handler func(*State) error) error {
	return compose.ProcessState(ctx, func(_ context.Context, s *State) error {
		if s == nil {
			return nil
		}
		return handler(s)
	})
}

func CheckpointIDOf(st *State) string {
	if st == nil {
		return ""
	}
	if st.Response != nil && strings.TrimSpace(st.Response.Trace.CheckpointID) != "" {
		return strings.TrimSpace(st.Response.Trace.CheckpointID)
	}
	return strings.TrimSpace(st.TraceID)
}
