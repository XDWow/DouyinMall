package returnexchange

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// GraphInput 子图入口显式入参；不挂 *domain.State 或 Recorder 等共享可变引用（Recorder 在需用时经 ProcessState 读取）。
type GraphInput struct {
	Message         string
	Intent          domain.Intent
	AwaitingConfirm bool
	Slots           map[string]any
	IntentEntities  map[string]string
}

// Output 子图对主图的契约。
type Output struct {
	FinalAnswer     string
	NeedHandoff     bool
	HandoffReason   string
	ReadOnly        bool
	AwaitingConfirm bool
	ToolMessages    []*schema.Message
	AwaitingUser    bool
	MissingSlots    []string
}

// InputFromState 从主图 State 投影为本子图入口快照；IntentEntities 做拷贝，避免子图内误改共享 map。
func InputFromState(st *domain.State) (GraphInput, error) {
	if st == nil {
		return GraphInput{}, fmt.Errorf("state is nil")
	}
	slots := cloneSlotsRE(st.Session.Slots)
	if slots == nil {
		slots = map[string]any{}
	}
	intentEntities := support.StringStringMapSnapshot(st.Intent.Entities)
	globalnode.ApplyIntentFieldsForTools(slots, intentEntities)
	globalnode.ResolveOrderRefFromTrustedRefs(slots, intentEntities, st.Session.CurrentRefs)
	return GraphInput{
		Message:         st.Input.Message,
		Intent:          st.Session.Intent,
		AwaitingConfirm: st.Session.AwaitingConfirm,
		Slots:           slots,
		IntentEntities:  intentEntities,
	}, nil
}
