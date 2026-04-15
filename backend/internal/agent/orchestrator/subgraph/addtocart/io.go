package addtocart

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// GraphInput 子图入口显式入参；Slots/Entities 均为拷贝，不挂 Session 引用。
type GraphInput struct {
	Slots    map[string]any
	Entities map[string]string
}

// Output 子图对主图的契约。
type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
	AwaitingUser  bool
	MissingSlots  []string
}

// InputFromState 从主图 State 投影为本子图入口快照；Entities 做拷贝，避免子图内误改共享 map。
func InputFromState(st *domain.State) (GraphInput, error) {
	if st == nil {
		return GraphInput{}, fmt.Errorf("state is nil")
	}
	slots := cloneSlotsCart(st.Session.Slots)
	entities := support.StringStringMapSnapshot(st.Intent.Entities)
	globalnode.ApplyIntentFieldsForTools(slots, entities)
	if slots == nil {
		slots = map[string]any{}
	}
	return GraphInput{Slots: slots, Entities: entities}, nil
}
