package tool

import (
	"context"
	"fmt"
	"strings"

	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
	cartservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	orderservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1"
	searchservice "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1/searchservice"
)

type RPCGateway struct {
	orderClient  orderservice.Client
	searchClient searchservice.Client
	cartClient   cartservice.Client
}

func NewRPCGateway(
	orderClient orderservice.Client,
	searchClient searchservice.Client,
	cartClient cartservice.Client,
) Gateway {
	return &RPCGateway{
		orderClient:  orderClient,
		searchClient: searchClient,
		cartClient:   cartClient,
	}
}

func (g *RPCGateway) QueryOrders(ctx context.Context, userID int64, orderID int64, limit int) ([]OrderSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	resp, err := g.orderClient.ListOrder(ctx, &orderv1.ListOrderReq{
		UserId: userID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}

	results := make([]OrderSummary, 0, len(resp.GetOrders()))
	for _, order := range resp.GetOrders() {
		if orderID > 0 && order.GetOrderId() != orderID {
			continue
		}
		results = append(results, OrderSummary{
			OrderID:     order.GetOrderId(),
			Status:      fmt.Sprintf("%d", order.GetOrderStatus()),
			TotalAmount: order.GetTotalAmount(),
			Currency:    order.GetCurrency(),
			CreatedAt:   unixMilliToTime(order.GetCreatedAt()),
		})
	}

	if orderID > 0 && len(results) == 0 {
		return nil, fmt.Errorf("order %d not found for current user", orderID)
	}
	return results, nil
}

func (g *RPCGateway) SearchProducts(ctx context.Context, query string, limit int) ([]ProductSummary, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 5
	}

	resp, err := g.searchClient.SearchProducts(ctx, &searchv1.SearchProductsReq{
		Keyword:         query,
		Page:            1,
		PageSize:        int64(limit),
		EnableHighlight: true,
	})
	if err != nil {
		return nil, err
	}

	results := make([]ProductSummary, 0, len(resp.GetProducts()))
	for _, product := range resp.GetProducts() {
		results = append(results, ProductSummary{
			ID:           product.GetId(),
			Name:         product.GetName(),
			Price:        product.GetPrice(),
			Categories:   product.GetCategories(),
			MerchantName: product.GetMerchantName(),
			InStock:      product.GetInStock(),
		})
	}
	return results, nil
}

func (g *RPCGateway) AddToCart(ctx context.Context, userID int64, productID int64, quantity int64) error {
	if productID <= 0 {
		return fmt.Errorf("product_id is required")
	}
	if quantity <= 0 {
		quantity = 1
	}

	productIDs := make([]int64, 0, quantity)
	for i := int64(0); i < quantity; i++ {
		productIDs = append(productIDs, productID)
	}

	_, err := g.cartClient.AddItem(ctx, &cartv1.AddItemReq{
		UserId:     userID,
		ProductIds: productIDs,
	})
	return err
}
