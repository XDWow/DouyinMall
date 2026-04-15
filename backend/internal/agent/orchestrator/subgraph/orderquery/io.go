package orderquery

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// GraphInput 子图入口显式入参（Eino 边类型）；不挂 Session/State 引用，仅快照字段。
type GraphInput struct {
	Slots map[string]any
}

// Output 子图对主图的契约。
type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
}

// InputFromState 从主图 State 投影为本子图入口快照（与共享 State 解耦）。
func InputFromState(st *domain.State) (GraphInput, error) {
	if st == nil {
		return GraphInput{}, fmt.Errorf("state is nil")
	}
	slots := cloneSlotsOrder(st.Session.Slots)
	intentEntities := support.StringStringMapSnapshot(st.Intent.Entities)
	globalnode.ApplyIntentFieldsForTools(slots, intentEntities)
	globalnode.ResolveOrderRefFromTrustedRefs(slots, intentEntities, st.Session.CurrentRefs)
	if slots == nil {
		slots = map[string]any{}
	}
	return GraphInput{Slots: slots}, nil
}
