package domain

type WorkflowRoute string

const (
	RouteUnknown             WorkflowRoute = "unknown"
	RouteOrderQuery          WorkflowRoute = "order_query"
	RouteInventory           WorkflowRoute = "inventory"
	RouteProductInfo         WorkflowRoute = "product_info"
	RouteAddToCart           WorkflowRoute = "add_to_cart"
	RouteReturnPolicy        WorkflowRoute = "return_policy"
	RouteReturnExchangeApply WorkflowRoute = "return_exchange_apply"
	RouteBaseQA              WorkflowRoute = "base_qa"
)
