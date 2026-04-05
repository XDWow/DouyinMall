package domain

type Intent string

const (
	IntentUnknown             Intent = "unknown"
	IntentOrderQuery          Intent = "order_query"
	IntentInventoryQuery      Intent = "inventory_query"
	IntentProductInfo         Intent = "product_info"
	IntentAddToCart           Intent = "add_to_cart"
	IntentReturnPolicy        Intent = "return_policy"
	IntentReturnExchangeApply Intent = "return_exchange_apply"
	IntentFallback            Intent = "fallback"
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
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusClosed SessionStatus = "closed"
	SessionStatusHuman  SessionStatus = "human"
)
