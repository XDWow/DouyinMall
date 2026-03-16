package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
)

// CartTool 购物车操作（add_to_cart + get_cart）
type CartTool struct {
	cartClient    cartservice.Client
	productClient productservice.Client // get_cart 需要补充商品名称和价格
}

func NewCartTool(cartClient cartservice.Client, productClient productservice.Client) *CartTool {
	return &CartTool{cartClient: cartClient, productClient: productClient}
}

// ==================== add_to_cart ====================

type addToCartArgs struct {
	UserID    int64 `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

// AddToCart 加入购物车
func (t *CartTool) AddToCart(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[addToCartArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.ProductID <= 0 {
		return mcp.NewErrorResult("product_id 无效")
	}
	if args.Quantity <= 0 {
		args.Quantity = 1
	}

	// AddItem 支持批量添加，此处只添加一个
	_, err = t.cartClient.AddItem(ctx, &cartv1.AddItemReq{
		UserId:     args.UserID,
		ProductIds: []int64{args.ProductID},
	})
	if err != nil {
		return mcp.NewErrorResult("加入购物车失败: " + err.Error())
	}

	// AddItemReq 不支持 quantity，如果用户要多件，逐次 IncrementQty
	for i := int64(1); i < args.Quantity; i++ {
		_, err = t.cartClient.IncrementQty(ctx, &cartv1.IncrementQtyReq{
			UserId:    args.UserID,
			ProductId: args.ProductID,
		})
		if err != nil {
			return mcp.NewErrorResult("设置数量失败: " + err.Error())
		}
	}

	return mcp.NewTextResult(toJSON(map[string]any{
		"product_id": args.ProductID,
		"quantity":   args.Quantity,
		"message":    "已加入购物车",
	}))
}

// ==================== get_cart ====================

type getCartArgs struct {
	UserID int64 `json:"user_id"`
}

// GetCart 获取购物车内容
func (t *CartTool) GetCart(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[getCartArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.UserID <= 0 {
		return mcp.NewErrorResult("user_id 无效")
	}

	resp, err := t.cartClient.GetCart(ctx, &cartv1.GetCartReq{UserId: args.UserID})
	if err != nil {
		return mcp.NewErrorResult("获取购物车失败: " + err.Error())
	}

	cart := resp.GetCart()
	if cart == nil || len(cart.GetItems()) == 0 {
		return mcp.NewTextResult("购物车为空。")
	}

	// 批量获取商品详情（名称、价格）
	productIDs := make([]int64, 0, len(cart.GetItems()))
	for _, item := range cart.GetItems() {
		productIDs = append(productIDs, item.GetProductId())
	}
	productMap := t.fetchProductInfo(ctx, productIDs)

	var totalPrice int64
	cartItems := make([]map[string]any, 0, len(cart.GetItems()))
	for _, item := range cart.GetItems() {
		pid := item.GetProductId()
		info := productMap[pid]
		subtotal := info.price * item.GetQuantity()
		totalPrice += subtotal

		cartItems = append(cartItems, map[string]any{
			"product_id": pid,
			"name":       info.name,
			"quantity":   item.GetQuantity(),
			"price":      fmt.Sprintf("%.2f", float64(info.price)/100),
		})
	}

	return mcp.NewTextResult(toJSON(map[string]any{
		"cart_items":  cartItems,
		"total_price": fmt.Sprintf("%.2f", float64(totalPrice)/100),
	}))
}

type productInfo struct {
	name  string
	price int64
}

func (t *CartTool) fetchProductInfo(ctx context.Context, ids []int64) map[int64]productInfo {
	result := make(map[int64]productInfo, len(ids))
	if len(ids) == 0 {
		return result
	}

	resp, err := t.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: ids})
	if err != nil {
		return result
	}
	for _, p := range resp.GetProduct() {
		result[p.GetId()] = productInfo{name: p.GetName(), price: p.GetPrice()}
	}
	return result
}
