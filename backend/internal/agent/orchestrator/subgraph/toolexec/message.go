package toolexec

import (
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

func CreateToolCallMessage(plans []domain.ToolCallPlan) (*schema.Message, error) {
	return support.BuildToolCallMessage(plans)
}
