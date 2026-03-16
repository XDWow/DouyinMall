package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
	checkoutv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
)

// CheckoutTool 下单（支持直接购买和购物车结算两种流程）
type CheckoutTool struct {
	checkoutClient checkoutservice.Client
	cartClient     cartservice.Client
}

func NewCheckoutTool(checkoutClient checkoutservice.Client, cartClient cartservice.Client) *CheckoutTool {
	return &CheckoutTool{checkoutClient: checkoutClient, cartClient: cartClient}
}

type createOrderArgs struct {
	UserID int64  `json:"user_id"`
	Source string `json:"source"` // "product" | "cart"

	// source=product 时使用
	Items []createOrderItem `json:"items,omitempty"`

	// source=cart 时：从购物车读取 items
}

type createOrderItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

// CreateOrder 创建订单
func (t *CheckoutTool) CreateOrder(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[createOrderArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.UserID <= 0 {
		return mcp.NewErrorResult("user_id 无效")
	}

	var checkoutItems []*checkoutv1.CheckoutItem

	switch args.Source {
	case "product":
		if len(args.Items) == 0 {
			return mcp.NewErrorResult("source=product 时 items 不能为空")
		}
		for _, item := range args.Items {
			if item.ProductID <= 0 || item.Quantity <= 0 {
				return mcp.NewErrorResult("items 中包含无效的 product_id 或 quantity")
			}
			checkoutItems = append(checkoutItems, &checkoutv1.CheckoutItem{
				ProductId: item.ProductID,
				Quantity:  item.Quantity,
			})
		}

	case "cart":
		// 从购物车服务读取当前内容
		cartResp, cartErr := t.cartClient.GetCart(ctx, &cartv1.GetCartReq{UserId: args.UserID})
		if cartErr != nil {
			return mcp.NewErrorResult("读取购物车失败: " + cartErr.Error())
		}
		cart := cartResp.GetCart()
		if cart == nil || len(cart.GetItems()) == 0 {
			return mcp.NewErrorResult("购物车为空，无法下单")
		}
		for _, ci := range cart.GetItems() {
			checkoutItems = append(checkoutItems, &checkoutv1.CheckoutItem{
				ProductId: ci.GetProductId(),
				Quantity:  ci.GetQuantity(),
			})
		}

	default:
		return mcp.NewErrorResult("source 必须是 \"product\" 或 \"cart\"")
	}

	// 调用 Checkout 服务下单
	resp, err := t.checkoutClient.PlaceOrder(ctx, &checkoutv1.PlaceOrderReq{
		UserId: args.UserID,
		Items:  checkoutItems,
	})
	if err != nil {
		return mcp.NewErrorResult("下单失败: " + err.Error())
	}

	return mcp.NewTextResult(toJSON(map[string]any{
		"order_id":    resp.GetOrderId(),
		"status":      "pending_payment",
		"total_price": fmt.Sprintf("%.2f", float64(resp.GetTotalAmount())/100),
		"payment_url": resp.GetPaymentUrl(),
	}))
}
