package tools

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
)

// Registry 工具注册表，将所有工具注册到 MCP Server
type Registry struct {
	search   *SearchTool
	product  *ProductTool
	cart     *CartTool
	checkout *CheckoutTool
	order    *OrderTool
}

func NewRegistry(
	search *SearchTool,
	product *ProductTool,
	cart *CartTool,
	checkout *CheckoutTool,
	order *OrderTool,
) *Registry {
	return &Registry{
		search:   search,
		product:  product,
		cart:     cart,
		checkout: checkout,
		order:    order,
	}
}

// Register 将 6 个工具注册到 MCP Server，覆盖完整电商购物流程
func (r *Registry) Register(server *mcp.Server) {

	// 1. search_products — 商品搜索
	server.RegisterTool(mcp.Tool{
		Name:        "search_products",
		Description: "Search products by keyword. Use when user wants to find or browse products.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":     map[string]any{"type": "string", "description": "Search keyword"},
				"page":      map[string]any{"type": "integer", "description": "Page number, default 1", "default": 1},
				"page_size": map[string]any{"type": "integer", "description": "Results per page, default 10", "default": 10},
			},
			"required": []string{"query"},
		}),
	}, r.search.SearchProducts)

	// 2. get_product_detail — 商品详情
	server.RegisterTool(mcp.Tool{
		Name:        "get_product_detail",
		Description: "Get complete product details including price, stock, description and images. Use when user asks about a specific product.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"product_id": map[string]any{"type": "integer", "description": "Product ID"},
			},
			"required": []string{"product_id"},
		}),
	}, r.product.GetProductDetail)

	// 3. add_to_cart — 加入购物车
	server.RegisterTool(mcp.Tool{
		Name:        "add_to_cart",
		Description: "Add a product to the user's cart. Use when user says 'add to cart', 'I want this', etc.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id":    map[string]any{"type": "integer", "description": "User ID"},
				"product_id": map[string]any{"type": "integer", "description": "Product ID"},
				"quantity":   map[string]any{"type": "integer", "description": "Quantity, default 1", "default": 1},
			},
			"required": []string{"user_id", "product_id"},
		}),
	}, r.cart.AddToCart)

	// 4. get_cart — 查看购物车
	server.RegisterTool(mcp.Tool{
		Name:        "get_cart",
		Description: "Retrieve the user's cart contents with product names and prices. Use when user asks 'what's in my cart' or before checkout.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "integer", "description": "User ID"},
			},
			"required": []string{"user_id"},
		}),
	}, r.cart.GetCart)

	// 5. create_order — 下单
	server.RegisterTool(mcp.Tool{
		Name:        "create_order",
		Description: "Create a new order. Supports two flows: source='product' for direct purchase (requires items), source='cart' for cart checkout.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "integer", "description": "User ID"},
				"source":  map[string]any{"type": "string", "enum": []string{"product", "cart"}, "description": "Purchase flow: 'product' for direct buy, 'cart' for cart checkout"},
				"items": map[string]any{
					"type":        "array",
					"description": "Required when source='product'. List of items to purchase.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"product_id": map[string]any{"type": "integer"},
							"quantity":   map[string]any{"type": "integer"},
						},
						"required": []string{"product_id", "quantity"},
					},
				},
			},
			"required": []string{"user_id", "source"},
		}),
	}, r.checkout.CreateOrder)

	// 6. get_order — 查看订单
	server.RegisterTool(mcp.Tool{
		Name:        "get_order",
		Description: "Retrieve order details by order ID. Use when user asks 'check my order', 'order status', 'where is my package'.",
		InputSchema: mcp.MustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"order_id": map[string]any{"type": "integer", "description": "Order ID"},
				"user_id":  map[string]any{"type": "integer", "description": "User ID"},
			},
			"required": []string{"order_id", "user_id"},
		}),
	}, r.order.GetOrder)
}

// ==================== 辅助 ====================

// parseArgs 解析 MCP 工具参数
func parseArgs[T any](arguments json.RawMessage) (T, error) {
	var args T
	err := json.Unmarshal(arguments, &args)
	return args, err
}

// toJSON 将结果序列化为 JSON 字符串
func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// wrapCtx 从 MCP arguments 中提取 user_id 并写入 context（供下游 RPC 鉴权用）
func wrapCtx(ctx context.Context, userID int64) context.Context {
	// TODO: 如需传递 user_id 到下游，可用 metadata.WithValue
	return ctx
}
