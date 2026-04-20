package domain

type Intent string

const (
	IntentProductService   Intent = "product_service"
	IntentOrderService     Intent = "order_service"
	IntentPromotionService Intent = "promotion_service"
	IntentAftersalesPolicy Intent = "aftersales_policy"
	IntentAftersalesApply  Intent = "aftersales_apply"
	IntentAddToCart        Intent = "add_to_cart"
	IntentUnknown          Intent = "unknown"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type ReplyStatus string

const (
	ReplyStatusAnswered ReplyStatus = "answered"
	ReplyStatusFallback ReplyStatus = "fallback"
	ReplyStatusHandoff  ReplyStatus = "handoff"
	ReplyStatusBlocked  ReplyStatus = "blocked"
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusClosed SessionStatus = "closed"
	SessionStatusHuman  SessionStatus = "human"
)
