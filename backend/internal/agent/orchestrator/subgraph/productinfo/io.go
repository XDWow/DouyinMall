package productinfo

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// GraphInput 子图流水线载体：标量/新 map 为入口快照；L1 写入 CachedResponse 时对 *ChatResult 解引用再取址，避免与 L1 返回值共用同一指针。
type GraphInput struct {
	TenantID     string
	UserID       int64
	SessionID    string
	TraceID      string
	CheckpointID string
	RawQuery     string
	History      []*schema.Message
	Intent       string
	SkillNames   []string
	IntentFields map[string]string

	CacheHit       bool
	HitLevel       string
	CachedResponse *domain.ChatResult // L1 命中且带正文时非 nil；由值拷贝得到，不引用 L1 内部对象
	L1Final        string
	Slots          map[string]any
	MissingSlots   []string
	Documents      []*schema.Document
	DocsText       string
	AgentFinal     string
	AgentTools     []*schema.Message
}

// Output 子图对主图的契约。
type Output struct {
	CacheHit      bool
	HitLevel      string
	Response      *domain.ChatResult
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
	Query         string
	Documents     []*schema.Document
	AwaitingUser  bool
	MissingSlots  []string
}

// InputFromState 从主图 State 投影为本子图入口快照。
func InputFromState(st *domain.State, skills *agentskill.Registry) (GraphInput, error) {
	if st == nil {
		return GraphInput{}, fmt.Errorf("state is nil")
	}
	ents := support.StringStringMapSnapshot(st.Intent.Entities)
	return GraphInput{
		TenantID:     st.Session.TenantID,
		UserID:       st.Input.UserID,
		SessionID:    st.Session.SessionID,
		TraceID:      st.TraceID,
		CheckpointID: st.Checkpoint,
		Slots:        cloneSlotsPI(st.Session.Slots),
		RawQuery:     st.Session.RawQuery,
		History:      append([]*schema.Message(nil), st.Session.Messages...),
		Intent:       string(st.Session.Intent),
		SkillNames:   subgraphmeta.FilteredSkillNames(st.Session.Route, skills),
		IntentFields: ents,
	}, nil
}
