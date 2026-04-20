package llm

import "github.com/cloudwego/eino/components/model"

type Tier string

const (
	TierWeak   Tier = "weak"
	TierStrong Tier = "strong"
)

type Router struct {
	weak             model.ToolCallingChatModel
	strong           model.ToolCallingChatModel
	enableDowngrade  bool
}

func NewRouter(weak, strong model.ToolCallingChatModel, enableDowngrade bool) *Router {
	if strong == nil {
		strong = weak
	}
	return &Router{
		weak:            weak,
		strong:          strong,
		enableDowngrade: enableDowngrade,
	}
}

func (r *Router) Weak() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	return r.weak
}

func (r *Router) Strong() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	if !r.enableDowngrade || r.strong == nil || r.weak == nil || r.strong == r.weak {
		return r.strong
	}
	return wrapFallbackChatModel(r.strong, r.weak)
}

func (r *Router) Select(tier Tier) model.ToolCallingChatModel {
	switch tier {
	case TierStrong:
		return r.Strong()
	default:
		return r.Weak()
	}
}
