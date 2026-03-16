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
// 由于 OrderService 没有 GetOrder RPC，使用 ListOrder + 客户端过滤
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

	resp, err := t.client.ListOrder(ctx, &orderv1.ListOrderReq{
		UserId: args.UserID,
		Limit:  50,
	})
	if err != nil {
		return mcp.NewErrorResult("查询订单失败: " + err.Error())
	}

	for _, o := range resp.GetOrders() {
		if o.GetOrderId() == args.OrderID {
			items := make([]map[string]any, 0, len(o.GetItems()))
			for _, item := range o.GetItems() {
				items = append(items, map[string]any{
					"product_id": item.GetProductId(),
					"quantity":   item.GetQuantity(),
					"price":      fmt.Sprintf("%.2f", float64(item.GetSnapshotPrice())/100),
				})
			}
			return mcp.NewTextResult(toJSON(map[string]any{
				"order_id":    o.GetOrderId(),
				"status":      orderStatusText(o.GetOrderStatus()),
				"items":       items,
				"total_price": fmt.Sprintf("%.2f", float64(o.GetTotalAmount())/100),
				"created_at":  o.GetCreatedAt(),
			}))
		}
	}

	return mcp.NewErrorResult("未找到该订单")
}

func orderStatusText(status uint32) string {
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
