package ai

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type EinoTier string

const (
	EinoTierWeak   EinoTier = "weak"
	EinoTierStrong EinoTier = "strong"
)

type EinoRouter struct {
	weak           model.ToolCallingChatModel
	strong         model.ToolCallingChatModel
	downgradeOne   bool
	retryTimes     int
}

func NewEinoRouter(weak, strong model.ToolCallingChatModel, downgradeOne bool, retryTimes int) *EinoRouter {
	if strong == nil {
		strong = weak
	}
	return &EinoRouter{
		weak:         weak,
		strong:       strong,
		downgradeOne: downgradeOne,
		retryTimes:   retryTimes,
	}
}

func (r *EinoRouter) Weak() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	return wrapRetryToolCallingChatModel(r.weak, r.retryTimes)
}

func (r *EinoRouter) Strong() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	strong := wrapRetryToolCallingChatModel(r.strong, r.retryTimes)
	if !r.downgradeOne || r.weak == nil || r.strong == nil || r.weak == r.strong {
		return strong
	}
	return &fallbackToolCallingChatModel{
		primary:  strong,
		fallback: wrapRetryToolCallingChatModel(r.weak, r.retryTimes),
	}
}

func (r *EinoRouter) Select(tier EinoTier) model.ToolCallingChatModel {
	switch tier {
	case EinoTierStrong:
		return r.Strong()
	default:
		return r.Weak()
	}
}

type fallbackToolCallingChatModel struct {
	primary  model.ToolCallingChatModel
	fallback model.ToolCallingChatModel
}

type retryToolCallingChatModel struct {
	base        model.ToolCallingChatModel
	retryTimes  int
}

func wrapRetryToolCallingChatModel(base model.ToolCallingChatModel, retryTimes int) model.ToolCallingChatModel {
	if base == nil || retryTimes <= 0 {
		return base
	}
	return &retryToolCallingChatModel{
		base:       base,
		retryTimes: retryTimes,
	}
}

func (m *retryToolCallingChatModel) Generate(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	var lastErr error
	for attempt := 0; attempt <= m.retryTimes; attempt++ {
		msg, err := m.base.Generate(ctx, input, opts...)
		if err == nil {
			return msg, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (m *retryToolCallingChatModel) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	var lastErr error
	for attempt := 0; attempt <= m.retryTimes; attempt++ {
		reader, err := m.base.Stream(ctx, input, opts...)
		if err == nil {
			return reader, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (m *retryToolCallingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return wrapRetryToolCallingChatModel(bound, m.retryTimes), nil
}

func (m *fallbackToolCallingChatModel) Generate(
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

func (m *fallbackToolCallingChatModel) Stream(
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

func (m *fallbackToolCallingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	primary, err := m.primary.WithTools(tools)
	if err != nil {
		return nil, err
	}
	fallback, err := m.fallback.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &fallbackToolCallingChatModel{
		primary:  primary,
		fallback: fallback,
	}, nil
}
