package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

// OrderTool 订单查询
type OrderTool struct {
	client orderservice.Client
}

func NewOrderTool(client orderservice.Client) *OrderTool {
	return &OrderTool{client: client}
}

type getOrderArgs struct {
	OrderID int64 `json:"order_id"`
	UserID  int64 `json:"user_id"`
}

// GetOrder 查询单个订单详情
func (t *OrderTool) GetOrder(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[getOrderArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.OrderID <= 0 {
		return mcp.NewErrorResult("order_id 无效")
	}
	if args.UserID <= 0 {
		return mcp.NewErrorResult("user_id 无效")
	}

	resp, err := t.client.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: args.OrderID})
	if err != nil {
		return mcp.NewErrorResult("查询订单失败: " + err.Error())
	}
	order := resp.GetOrder()
	if order == nil {
		return mcp.NewErrorResult("未找到该订单")
	}

	if order.GetUserId() != args.UserID {
		return mcp.NewErrorResult("订单不属于当前用户")
	}

	items := make([]map[string]any, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, map[string]any{
			"product_id": item.GetProductId(),
			"quantity":   item.GetQuantity(),
			"price":      fmt.Sprintf("%.2f", float64(item.GetSnapshotPrice())/100),
		})
	}

	return mcp.NewTextResult(toJSON(map[string]any{
		"order_id":    order.GetOrderId(),
		"status":      orderStatusText(order.GetOrderStatus()),
		"remark":      order.GetRemark(),
		"items":       items,
		"total_price": fmt.Sprintf("%.2f", float64(order.GetTotalAmount())/100),
		"created_at":  order.GetCreatedAt(),
	}))
}

func orderStatusText(status orderv1.OrderStatus) string {
	switch status {
	case 0:
		return "pending_payment"
	case 1:
		return "paid"
	case 2:
		return "cancelled"
	case 3:
		return "cancelled"
	default:
		return "unknown"
	}
}
