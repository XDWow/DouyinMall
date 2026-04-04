package domain

type Intent string

const (
	IntentUnknown             Intent = "unknown"
	IntentFAQ                 Intent = "faq"
	IntentProductSearch       Intent = "product_search"
	IntentOrderQuery          Intent = "order_query"
	IntentAddToCart           Intent = "add_to_cart"
	IntentPolicy              Intent = "policy"
	IntentComplaint           Intent = "complaint"
	IntentHandoff             Intent = "handoff"
	IntentChitchat            Intent = "chitchat"
	IntentUnsupported         Intent = "unsupported"
	IntentReturnPolicy        Intent = "return_policy"
	IntentInventoryQuery      Intent = "inventory_query"
	IntentProductInfo         Intent = "product_info"
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
