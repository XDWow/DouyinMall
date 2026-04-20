package understanding

type UnderstandingInput struct {
	UserMessage   string
	RecentHistory []string
}

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

type UnderstandingResult struct {
	Intent         Intent         `json:"intent"`
	RewrittenQuery string         `json:"rewritten_query,omitempty"`
	Slots          map[string]any `json:"slots,omitempty"`
}
