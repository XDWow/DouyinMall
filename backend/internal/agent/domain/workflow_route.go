package domain

type WorkflowRoute string

const (
	RouteProductService   WorkflowRoute = "product_service"
	RouteOrderService     WorkflowRoute = "order_service"
	RoutePromotionService WorkflowRoute = "promotion_service"
	RouteAftersalesPolicy WorkflowRoute = "aftersales_policy"
	RouteAftersalesApply  WorkflowRoute = "aftersales_apply"
	RouteAddToCart        WorkflowRoute = "add_to_cart"
	RouteUnknown          WorkflowRoute = "unknown"
)

func WorkflowRouteFromIntent(intent Intent) WorkflowRoute {
	switch intent {
	case IntentProductService:
		return RouteProductService
	case IntentOrderService:
		return RouteOrderService
	case IntentPromotionService:
		return RoutePromotionService
	case IntentAftersalesPolicy:
		return RouteAftersalesPolicy
	case IntentAftersalesApply:
		return RouteAftersalesApply
	case IntentAddToCart:
		return RouteAddToCart
	default:
		return RouteUnknown
	}
}

func DefaultReadOnlyForIntent(intent Intent) bool {
	switch intent {
	case IntentAddToCart, IntentAftersalesApply:
		return false
	default:
		return true
	}
}
