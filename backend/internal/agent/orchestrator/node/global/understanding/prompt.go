package understanding

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

//go:embed system_prompt.md
var systemPrompt string

func BuildDefaultMessages(input UnderstandingInput) ([]*schema.Message, error) {
	msg := strings.TrimSpace(input.UserMessage)
	if msg == "" {
		return nil, fmt.Errorf("user message is empty")
	}

	history := strings.TrimSpace(strings.Join(input.RecentHistory, "\n"))
	if history == "" {
		history = "（无）"
	}

	// 模板文件只保存静态规则；运行时把历史消息和当前消息填进来，
	// 最终发给模型的是两条 chat messages。
	userBlock := strings.TrimSpace(`
<conversation_history>
` + history + `
</conversation_history>

<current_user_message>
` + msg + `
</current_user_message>
`)
	return []*schema.Message{
		schema.SystemMessage(strings.TrimSpace(systemPrompt)),
		schema.UserMessage(userBlock),
	}, nil
}
