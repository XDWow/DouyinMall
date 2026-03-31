package grpc

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
)

type OrderHandler struct {
	createOrderUC       *usecase.CreateOrderUseCase
	getOrderUC          *usecase.GetOrderUseCase
	listUserOrderUC     *usecase.ListUserOrderUseCase
	changeOrderStatusUC *usecase.ChangeOrderStatusUseCase
}

func NewOrderHandler(
	createOrderUC *usecase.CreateOrderUseCase,
	getOrderUC *usecase.GetOrderUseCase,
	listUserOrderUC *usecase.ListUserOrderUseCase,
	changeOrderStatusUC *usecase.ChangeOrderStatusUseCase,
) *OrderHandler {
	return &OrderHandler{
		createOrderUC:       createOrderUC,
		getOrderUC:          getOrderUC,
		listUserOrderUC:     listUserOrderUC,
		changeOrderStatusUC: changeOrderStatusUC,
	}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderReq) (*orderv1.CreateOrderResp, error) {
	items := make([]domain.OrderItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		items = append(items, domain.OrderItem{
			ProductID:        item.GetProductId(),
			SKUID:            item.GetSkuId(),
			Quantity:         item.GetQuantity(),
			SnapshotPrice:    item.GetSnapshotPrice(),
			SnapshotCurrency: item.GetSnapshotCurrency(),
			Price:            item.GetConvertedPrice(),
		})
	}

	address := domain.Address{}
	if req.GetAddress() != nil {
		address = domain.Address{
			Street:  req.GetAddress().GetStreetAddress(),
			City:    req.GetAddress().GetCity(),
			State:   req.GetAddress().GetState(),
			Country: req.GetAddress().GetCountry(),
			Zipcode: req.GetAddress().GetZipCode(),
			Phone:   req.GetAddress().GetPhone(),
		}
	}

	id, err := h.createOrderUC.Execute(ctx, usecase.CreateOrderCmd{
		OrderID:       req.GetOrderId(),
		UserID:        req.GetUserId(),
		Currency:      req.GetCurrency(),
		Remark:        req.GetRemark(),
		Address:       address,
		OrderKind:     req.GetOrderKind(),
		ActivityID:    req.GetActivityId(),
		PayableAmount: req.GetPayableAmount(),
		Items:         items,
	})
	if err != nil {
		return nil, err
	}
	return &orderv1.CreateOrderResp{OrderId: id}, nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderReq) (*orderv1.GetOrderResp, error) {
	order, err := h.getOrderUC.Execute(ctx, usecase.GetOrderCmd{OrderID: req.GetOrderId()})
	if err != nil {
		return nil, err
	}
	return &orderv1.GetOrderResp{Order: toProtoOrder(order)}, nil
}

func (h *OrderHandler) ChangeOrderStatus(ctx context.Context, req *orderv1.ChangeOrderStatusReq) (*orderv1.ChangeOrderStatusResp, error) {
	cmd := usecase.ChangeOrderStatusCmd{
		OrderID: req.GetOrderId(),
		Action:  domain.OrderAction(req.GetAction()),
	}
	result, err := h.changeOrderStatusUC.Execute(ctx, cmd)
	return &orderv1.ChangeOrderStatusResp{Changed: result.Changed}, err
}

func (h *OrderHandler) ListOrder(ctx context.Context, req *orderv1.ListOrderReq) (*orderv1.ListOrderResp, error) {
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
		orders[i] = toProtoOrder(o)
	}
	return &orderv1.ListOrderResp{
		Orders:     orders,
		NextCursor: resp.NextCursor,
	}, nil
}

func toProtoOrder(o *domain.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, len(o.OrderItems))
	for i, item := range o.OrderItems {
		items[i] = &orderv1.OrderItem{
			ProductId:        item.ProductID,
			SkuId:            item.SKUID,
			Quantity:         item.Quantity,
			SnapshotPrice:    item.SnapshotPrice,
			SnapshotCurrency: item.SnapshotCurrency,
			ConvertedPrice:   item.Price,
		}
	}

	return &orderv1.Order{
		OrderId:     o.ID,
		UserId:      o.UserID,
		OrderStatus: orderv1.OrderStatus(o.Status),
		Items:       items,
		TotalAmount: o.PayableAmount.Total,
		Currency:    o.PayableAmount.Currency,
		Address: &orderv1.Address{
			StreetAddress: o.Addr.Street,
			City:          o.Addr.City,
			State:         o.Addr.State,
			Country:       o.Addr.Country,
			ZipCode:       o.Addr.Zipcode,
			Phone:         o.Addr.Phone,
		},
		Remark:     o.Remark,
		CreatedAt:  o.CreatedAt.Unix(),
		ExpireAt:   o.ExpireAt.Unix(),
		OrderKind:  o.OrderKind,
		ActivityId: o.ActivityID,
	}
}
