package state

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
)

func NewFlowContext(req dto.ChatRequest, writer StreamWriter, opts InitOptions) *FlowContext {
	traceID := uuid.NewString()
	state := SessionState{
		SessionID:    req.SessionID,
		UserID:       req.UserID,
		RawQuery:     strings.TrimSpace(req.Message),
		Intent:       dto.IntentUnknown,
		Route:        RouteUnknown,
		Slots:        map[string]any{},
		KBVersion:    opts.KBVersion,
		FeatureFlags: opts.FeatureFlags,
		ReadOnly:     true,
		ResumeFromCP: strings.TrimSpace(req.ResumeToken) != "",
	}
	return &FlowContext{
		StartedAt:    time.Now(),
		TraceID:      traceID,
		Request:      req,
		StreamWriter: writer,
		Recorder:     agenttool.NewSafeExecutionRecorder(),
		State:        state,
		Response: &dto.ChatResponse{
			SessionID:   req.SessionID,
			TraceID:     traceID,
			Status:      dto.ReplyStatusFallback,
			Intent:      dto.IntentUnknown,
			Trace:       dto.Trace{TraceID: traceID},
			Confidence:  0,
			NeedHandoff: false,
		},
	}
}

func (f *FlowContext) EnsureResponse() *dto.ChatResponse {
	if f.Response == nil {
		f.Response = &dto.ChatResponse{
			SessionID:   f.Request.SessionID,
			TraceID:     f.TraceID,
			Status:      dto.ReplyStatusFallback,
			Intent:      dto.IntentUnknown,
			Trace:       dto.Trace{TraceID: f.TraceID},
			Confidence:  0,
			NeedHandoff: false,
		}
	}
	if f.Response.Trace.TraceID == "" {
		f.Response.Trace.TraceID = f.TraceID
	}
	return f.Response
}

func (f *FlowContext) ToolExecutions() []dto.ToolExecution {
	if f == nil || f.Recorder == nil {
		return nil
	}
	return f.Recorder.Snapshot()
}
