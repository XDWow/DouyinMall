package understanding

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type UnderstandingNode struct {
	model model.ToolCallingChatModel
}

func NewUnderstandingNode(chatModel model.ToolCallingChatModel) *UnderstandingNode {
	return &UnderstandingNode{model: chatModel}
}

func (n *UnderstandingNode) Invoke(ctx context.Context, input UnderstandingInput) (*UnderstandingResult, error) {
	if n == nil || n.model == nil {
		return fallbackUnderstandingResult(), nil
	}
	messages, err := BuildDefaultMessages(input)
	if err != nil || len(messages) == 0 {
		return fallbackUnderstandingResult(), nil
	}
	msg, err := n.model.Generate(ctx, messages,
		model.WithTemperature(0),
		model.WithMaxTokens(256),
		model.WithToolChoice(schema.ToolChoiceForbidden),
	)
	if err != nil || msg == nil {
		return fallbackUnderstandingResult(), nil
	}
	return ParseUnderstandingResult(msg.Content), nil
}
