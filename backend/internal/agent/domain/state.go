package domain

import (
	"strings"
	"time"
)

// State 多节点共享的编排上下文（可读写）。本轮用户入参单独放在 Input，不把「整份 State」当作节点的业务输出类型——下游从同一 *State 读即可。
// 各节点业务上的入参/出参仍用专用 XXXInput / 返回值表达数据流；图用 *State 仅为传递共享指针。
type State struct {
	Input TurnInput `json:"input"`

	StartedAt        time.Time         `json:"-"`
	TraceID          string            `json:"trace_id"`
	PersistedSession *Session          `json:"session_meta,omitempty"`
	Response         *ChatResult       `json:"response,omitempty"`
	Checkpoint       string            `json:"checkpoint,omitempty"`
	Interrupt        *InterruptState   `json:"interrupt,omitempty"`
	StreamWriter     StreamWriter      `json:"-"`
	Recorder         ToolExecutionSink `json:"-"`
	Session          Session           `json:"session"`
	Cache            CacheState        `json:"cache"`
	Intent           IntentResult      `json:"intent"`
	Rewrite          RewriteResult     `json:"rewrite"`
	Retrieval        RetrievalResult   `json:"retrieval"`
	Tool             ToolState         `json:"tool"`
	Answer           AnswerResult      `json:"answer"`
}

// EnsureChatResult 懒创建 st.Response（合并 Input 与 Session 上的 SessionID）。
func EnsureChatResult(in TurnInput, st *State) *ChatResult {
	if st == nil {
		return nil
	}
	if st.Response == nil {
		sid := strings.TrimSpace(in.SessionID)
		if sid == "" {
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

func (s *State) ToolExecutions() []ToolExecution {
	if s == nil || s.Recorder == nil {
		return nil
	}
	return s.Recorder.Snapshot()
}
