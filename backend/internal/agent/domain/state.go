package domain

import "strings"

// State is the only shared graph-level context.
type State struct {
	Input          *ChatInput  `json:"input"`
	TraceID        string      `json:"trace_id"`
	Session        *Session    `json:"session"`
	Intent         Intent      `json:"intent"`
	RewrittenQuery string      `json:"rewritten_query,omitempty"`
	Response       *ChatResult `json:"response,omitempty"`
}

func EnsureChatResult(in *ChatInput, st *State) *ChatResult {
	if st == nil {
		return nil
	}
	if st.Response == nil {
		sid := ""
		if in != nil {
			sid = strings.TrimSpace(in.SessionID)
		}
		if sid == "" && st.Session != nil {
			sid = strings.TrimSpace(st.Session.SessionID)
		}
		st.Response = &ChatResult{
			SessionID:   sid,
			TraceID:     st.TraceID,
			Status:      ReplyStatusFallback,
			Intent:      IntentUnknown,
			Trace:       Trace{TraceID: st.TraceID},
			Confidence:  0,
			NeedHandoff: false,
		}
	}
	if st.Response.Trace.TraceID == "" {
		st.Response.Trace.TraceID = st.TraceID
	}
	return st.Response
}

func (s *State) EnsureResponse() *ChatResult {
	if s == nil {
		return nil
	}
	return EnsureChatResult(s.Input, s)
}

func (s *State) WorkflowRoute() WorkflowRoute {
	if s == nil {
		return RouteUnknown
	}
	return WorkflowRouteFromIntent(s.Intent)
}
