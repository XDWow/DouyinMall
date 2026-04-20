package llm

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fallbackChatModel struct {
	primary  model.ToolCallingChatModel
	fallback model.ToolCallingChatModel
}

func wrapFallbackChatModel(primary, fallback model.ToolCallingChatModel) model.ToolCallingChatModel {
	if primary == nil {
		return fallback
	}
	if fallback == nil || primary == fallback {
		return primary
	}
	return &fallbackChatModel{
		primary:  primary,
		fallback: fallback,
	}
}

func (m *fallbackChatModel) Generate(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	msg, err := m.primary.Generate(ctx, input, opts...)
	if err == nil {
		return msg, nil
	}
	return m.fallback.Generate(ctx, input, opts...)
}

func (m *fallbackChatModel) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	reader, err := m.primary.Stream(ctx, input, opts...)
	if err == nil {
		return reader, nil
	}
	return m.fallback.Stream(ctx, input, opts...)
}

func (m *fallbackChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	primary, err := m.primary.WithTools(tools)
	if err != nil {
		return nil, err
	}
	fallback, err := m.fallback.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return wrapFallbackChatModel(primary, fallback), nil
}
