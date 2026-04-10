package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
)

type fetchSkillIn struct {
	SkillName string `json:"skill_name"`
}

// NewFetchSkillTool 注册「按名拉取技能全文」工具；skill_name 必须与 [WithSkillWhitelist] 注入的列表一致。
func NewFetchSkillTool(skills *agentskill.Registry) (einotool.InvokableTool, error) {
	if skills == nil {
		return nil, nil
	}
	return toolutils.InferTool(
		"fetch_skill",
		"根据技能名称从技能库返回完整正文（查资料）。skill_name 必须是当前对话业务允许列表中的名称，不可编造。",
		func(ctx context.Context, in fetchSkillIn) (string, error) {
			name := strings.TrimSpace(in.SkillName)
			if name == "" {
				return "", fmt.Errorf("skill_name 不能为空")
			}
			if !skillNameAllowed(ctx, name) {
				return "", fmt.Errorf("技能 %q 不在当前业务允许列表中", name)
			}
			items := skills.Load([]string{name})
			if len(items) == 0 {
				return "", fmt.Errorf("未找到技能 %q", name)
			}
			body := strings.TrimSpace(items[0].Body)
			if body == "" {
				return fmt.Sprintf("技能 %q 暂无正文", name), nil
			}
			return body, nil
		},
	)
}
