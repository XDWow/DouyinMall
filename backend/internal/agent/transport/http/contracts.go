package http

import (
	"encoding/json"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentusecase "github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
)

type chatRequest struct {
	SessionID      string            `json:"session_id"`
	UserID         int64             `json:"user_id"`
	Message        string            `json:"message"`
	ResumeToken    string            `json:"resume_token,omitempty"`
	InterruptID    string            `json:"interrupt_id,omitempty"`
	ResumeDataJSON string            `json:"resume_data_json,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type workflowResumeRequest struct {
	CheckpointID string          `json:"checkpoint_id"`
	InterruptID  string          `json:"interrupt_id"`
	ResumeData   json.RawMessage `json:"resume_data,omitempty"`
	UserID       int64           `json:"user_id,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
}

type createSessionRequest struct {
	UserID int64 `json:"user_id"`
}

type chatResponse struct {
	SessionID      string             `json:"session_id"`
	TraceID        string             `json:"trace_id"`
	Status         domain.ReplyStatus `json:"status"`
	Reply          string             `json:"reply"`
	Intent         domain.Intent      `json:"intent"`
	Confidence     float64            `json:"confidence"`
	NeedHandoff    bool               `json:"need_handoff"`
	HandoffReason  string             `json:"handoff_reason,omitempty"`
	References     []knowledgeRef     `json:"references,omitempty"`
	UsedToolNames  []string           `json:"used_tool_names,omitempty"`
	ToolExecutions []toolExecution    `json:"tool_executions,omitempty"`
	Trace          traceResponse      `json:"trace"`
	Interrupt      *interruptInfo     `json:"interrupt,omitempty"`
	Interrupted    bool               `json:"interrupted,omitempty"`
}

type sessionResponse struct {
	SessionID   string               `json:"session_id"`
	UserID      int64                `json:"user_id"`
	Status      domain.SessionStatus `json:"status"`
	LastMessage string               `json:"last_message,omitempty"`
	TotalTurns  int                  `json:"total_turns"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

type historyResponse struct {
	Messages []messageResponse `json:"messages"`
	Total    int               `json:"total"`
}

type listSessionsResponse struct {
	Sessions []sessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

type messageResponse struct {
	ID         string        `json:"id"`
	SessionID  string        `json:"session_id"`
	Role       domain.Role   `json:"role"`
	Content    string        `json:"content"`
	Intent     domain.Intent `json:"intent,omitempty"`
	Confidence float64       `json:"confidence,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

type knowledgeRef struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Snippet  string            `json:"snippet"`
	Category string            `json:"category"`
	Score    float64           `json:"score"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type toolExecution struct {
	Name       string            `json:"name"`
	Arguments  map[string]any    `json:"arguments,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Success    bool              `json:"success"`
	Result     string            `json:"result,omitempty"`
	Error      string            `json:"error,omitempty"`
	LatencyMs  int64             `json:"latency_ms"`
	OccurredAt time.Time         `json:"occurred_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type traceResponse struct {
	TraceID        string      `json:"trace_id"`
	CheckpointID   string      `json:"checkpoint_id,omitempty"`
	CacheHit       bool        `json:"cache_hit"`
	RewrittenQuery string      `json:"rewritten_query,omitempty"`
	Steps          []traceStep `json:"steps,omitempty"`
}

type traceStep struct {
	Node      string `json:"node"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Detail    string `json:"detail,omitempty"`
}

type interruptInfo struct {
	CheckpointID  string         `json:"checkpoint_id"`
	InterruptID   string         `json:"interrupt_id,omitempty"`
	RerunNodes    []string       `json:"rerun_nodes,omitempty"`
	InterruptInfo map[string]any `json:"interrupt_info,omitempty"`
}

func toChatInput(req chatRequest) agentusecase.ChatInput {
	return agentusecase.ChatInput{
		SessionID:      req.SessionID,
		UserID:         req.UserID,
		Message:        req.Message,
		ResumeToken:    req.ResumeToken,
		InterruptID:    req.InterruptID,
		ResumeDataJSON: req.ResumeDataJSON,
		Metadata:       req.Metadata,
	}
}

func toCreateSessionInput(req createSessionRequest) agentusecase.CreateSessionInput {
	return agentusecase.CreateSessionInput{UserID: req.UserID}
}

func toChatResponse(out *agentusecase.ChatOutput) *chatResponse {
	if out == nil {
		return &chatResponse{}
	}

	refs := make([]knowledgeRef, 0, len(out.References))
	for _, ref := range out.References {
		refs = append(refs, knowledgeRef{
			ID:       ref.ID,
			Title:    ref.Title,
			Snippet:  ref.Snippet,
			Category: ref.Category,
			Score:    ref.Score,
			Metadata: ref.Metadata,
		})
	}

	execs := make([]toolExecution, 0, len(out.UsedToolNames))
	for _, name := range out.UsedToolNames {
		execs = append(execs, toolExecution{Name: name, Success: true})
	}

	steps := make([]traceStep, 0, len(out.Trace.Steps))
	for _, step := range out.Trace.Steps {
		steps = append(steps, traceStep{
			Node:      step.Node,
			Status:    step.Status,
			LatencyMs: step.LatencyMs,
			Detail:    step.Detail,
		})
	}

	resp := &chatResponse{
		SessionID:      out.SessionID,
		TraceID:        out.TraceID,
		Status:         out.Status,
		Reply:          out.Reply,
		Intent:         out.Intent,
		Confidence:     out.Confidence,
		NeedHandoff:    out.NeedHandoff,
		HandoffReason:  out.HandoffReason,
		References:     refs,
		UsedToolNames:  append([]string(nil), out.UsedToolNames...),
		ToolExecutions: execs,
		Trace: traceResponse{
			TraceID:        out.Trace.TraceID,
			CheckpointID:   out.Trace.CheckpointID,
			CacheHit:       out.Trace.CacheHit,
			RewrittenQuery: out.Trace.RewrittenQuery,
			Steps:          steps,
		},
	}
	if out.Interrupt != nil {
		resp.Interrupt = &interruptInfo{
			CheckpointID:  out.Interrupt.CheckpointID,
			InterruptID:   out.Interrupt.InterruptID,
			RerunNodes:    append([]string(nil), out.Interrupt.RerunNodes...),
			InterruptInfo: out.Interrupt.Detail,
		}
	}
	resp.Interrupted = out.Interrupted
	return resp
}

func toSessionResponse(out *agentusecase.SessionOutput) *sessionResponse {
	if out == nil {
		return &sessionResponse{}
	}
	return &sessionResponse{
		SessionID:   out.SessionID,
		UserID:      out.UserID,
		Status:      out.Status,
		LastMessage: out.LastMessage,
		TotalTurns:  out.TotalTurns,
		CreatedAt:   out.CreatedAt,
		UpdatedAt:   out.UpdatedAt,
	}
}

func toHistoryResponse(out *agentusecase.HistoryOutput) *historyResponse {
	if out == nil {
		return &historyResponse{}
	}
	items := make([]messageResponse, 0, len(out.Messages))
	for _, msg := range out.Messages {
		items = append(items, messageResponse{
			ID:         msg.ID,
			SessionID:  msg.SessionID,
			Role:       msg.Role,
			Content:    msg.Content,
			Intent:     msg.Intent,
			Confidence: msg.Confidence,
			CreatedAt:  msg.CreatedAt,
		})
	}
	return &historyResponse{Messages: items, Total: out.Total}
}

func toListSessionsResponse(out *agentusecase.SessionListOutput) *listSessionsResponse {
	if out == nil {
		return &listSessionsResponse{}
	}
	items := make([]sessionResponse, 0, len(out.Sessions))
	for _, session := range out.Sessions {
		items = append(items, *toSessionResponse(&session))
	}
	return &listSessionsResponse{Sessions: items, Total: out.Total}
}
