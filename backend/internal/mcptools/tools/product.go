package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
)

// ProductTool 商品详情查询（单个商品）
type ProductTool struct {
	client productservice.Client
}

func NewProductTool(client productservice.Client) *ProductTool {
	return &ProductTool{client: client}
}

type getProductDetailArgs struct {
	ProductID int64 `json:"product_id"`
}

// GetProductDetail 获取单个商品完整详情
func (t *ProductTool) GetProductDetail(ctx context.Context, arguments json.RawMessage) *mcp.CallToolResult {
	args, err := parseArgs[getProductDetailArgs](arguments)
	if err != nil {
		return mcp.NewErrorResult("参数解析失败: " + err.Error())
	}
	if args.ProductID <= 0 {
		return mcp.NewErrorResult("product_id 无效")
	}

	resp, err := t.client.GetProducts(ctx, &productv1.GetProductsReq{
		Id: []int64{args.ProductID},
	})
	if err != nil {
		return mcp.NewErrorResult("查询商品失败: " + err.Error())
	}

	products := resp.GetProduct()
	if len(products) == 0 {
		return mcp.NewErrorResult("商品不存在")
	}

	p := products[0]
	return mcp.NewTextResult(toJSON(map[string]any{
		"product_id":  p.GetId(),
		"name":        p.GetName(),
		"price":       fmt.Sprintf("%.2f", float64(p.GetPrice())/100),
		"stock":       boolToStock(p.GetInStock()),
		"description": p.GetDescription(),
		"images":      p.GetSliderImgs(),
		"categories":  p.GetCategories(),
	}))
}

func boolToStock(inStock bool) string {
	if inStock {
		return "有货"
	}
	return "缺货"
}
