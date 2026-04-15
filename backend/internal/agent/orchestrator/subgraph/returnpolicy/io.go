package returnpolicy

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
)

// GraphInput 子图入口显式入参；History 等为快照，不挂 *domain.State。
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

	CacheHit   bool
	HitLevel   string
	Response   *domain.ChatResult
	L1Final    string
	Query      string
	Documents  []*schema.Document
	AgentFinal string
}

// Output 子图对主图的契约。
type Output struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Query       string
	Documents   []*schema.Document
}

// InputFromState 从主图 State 投影为本子图入口快照。
func InputFromState(st *domain.State, skills *agentskill.Registry) (GraphInput, error) {
	if st == nil {
		return GraphInput{}, fmt.Errorf("state is nil")
	}
	return GraphInput{
		TenantID:     st.Session.TenantID,
		UserID:       st.Input.UserID,
		SessionID:    st.Session.SessionID,
		TraceID:      st.TraceID,
		CheckpointID: st.Checkpoint,
		RawQuery:     st.Session.RawQuery,
		History:      append([]*schema.Message(nil), st.Session.Messages...),
		Intent:       string(st.Session.Intent),
		SkillNames:   subgraphmeta.FilteredSkillNames(st.Session.Route, skills),
	}, nil
}
