package grpc

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
)

// OrderHandler 依赖它需要暴露的usecase，而不是所有usecase
// 每个RPC方法对应一个usecase，保持单一职责
type OrderHandler struct {
	createOrderUC       *usecase.CreateOrderUseCase
	listUserOrderUC     *usecase.ListUserOrderUseCase
	changeOrderStatusUC *usecase.ChangeOrderStatusUseCase
	// 批量取消是内部job调用的，不需要暴露为RPC
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderReq) (res *orderv1.CreateOrderResp, err error) {
	var items []domain.OrderItem
	for _, item := range req.GetItems() {
		items = append(items, domain.OrderItem{
			ProductID:        item.ProductId,
			Quantity:         item.Quantity,
			SnapshotPrice:    item.SnapshotPrice,
			SnapshotCurrency: item.SnapshotCurrency,
			Price:            item.ConvertedPrice,
		})
	}
	cmd := usecase.CreateOrderCmd{
		OrderID:  req.GetOrderId(),
		UserID:   req.GetUserId(),
		Currency: req.GetCurrency(),
		Phone:    req.GetPhone(),
		Address: domain.Address{
			Street:  req.GetAddress().StreetAddress,
			City:    req.GetAddress().City,
			State:   req.GetAddress().State,
			Country: req.GetAddress().Country,
			Zipcode: req.GetAddress().ZipCode,
		},
		Items: items,
	}
	id, err := h.createOrderUC.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return &orderv1.CreateOrderResp{OrderId: id}, nil
}

func (h *OrderHandler) ChangeOrderStatus(ctx context.Context, req *orderv1.ChangeOrderStatusReq) (res *orderv1.ChangeOrderStatusResp, err error) {
	cmd := usecase.ChangeOrderStatusCmd{
		OrderID:     req.GetOrderId(),
		OrderStatus: domain.OrderStatus(req.GetOrderStatus()),
	}
	err = h.changeOrderStatusUC.Execute(ctx, cmd)
	return &orderv1.ChangeOrderStatusResp{}, err
}

func (h *OrderHandler) ListOrder(ctx context.Context, req *orderv1.ListOrderReq) (res *orderv1.ListOrderResp, err error) {
	cmd := usecase.ListUserOrderCmd{
		UserID: req.GetUserId(),
		Cursor: req.GetCursor(),
		Limit:  req.GetLimit(),
	}
	resp, err := h.listUserOrderUC.Execute(cmd)
	if err != nil {
		return nil, err
	}

	orders := make([]*orderv1.Order, len(resp.Orders))
	for i, o := range resp.Orders {
		items := make([]*orderv1.OrderItem, len(o.OrderItems))
		for j, item := range o.OrderItems {
			items[j] = &orderv1.OrderItem{
				ProductId:        item.ProductID,
				Quantity:         item.Quantity,
				SnapshotPrice:    item.SnapshotPrice,
				SnapshotCurrency: item.SnapshotCurrency,
				ConvertedPrice:   item.Price,
			}
		}
		orders[i] = &orderv1.Order{
			OrderId:     o.ID,
			UserId:      o.UserID,
			OrderStatus: uint32(o.Status.AsUint8()),
			Items:       items,
			TotalAmount: o.PayableAmount.Total,
			Currency:    o.PayableAmount.Currency,
			Address: &orderv1.Address{
				StreetAddress: o.Addr.Street,
				City:          o.Addr.City,
				State:         o.Addr.State,
				Country:       o.Addr.Country,
				ZipCode:       o.Addr.Zipcode,
			},
			CreatedAt: o.CreatedAt.Unix(),
		}
	}
	return &orderv1.ListOrderResp{Orders: orders}, nil
}

func NewOrderHandler(
	createOrderUC *usecase.CreateOrderUseCase,
	listUserOrderUC *usecase.ListUserOrderUseCase,
	changeOrderStatusUC *usecase.ChangeOrderStatusUseCase,
) *OrderHandler {
	return &OrderHandler{
		createOrderUC:       createOrderUC,
		listUserOrderUC:     listUserOrderUC,
		changeOrderStatusUC: changeOrderStatusUC,
	}
}
