package handler

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/cart/domain"
	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
)

type CartHandler struct {
	CartService service.CartService
}

func NewCartHandler(cartService service.CartService) *CartHandler {
	return &CartHandler{CartService: cartService}
}

func (h *CartHandler) AddItem(ctx context.Context, req *cartv1.AddItemReq) (*cartv1.AddItemResp, error) {
	items := make([]domain.CartItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		items = append(items, domain.CartItem{
			ProductID: item.GetProductId(),
			SKUID:     item.GetSkuId(),
			Quantity:  item.GetQuantity(),
		})
	}
	if err := h.CartService.AddItems(ctx, req.GetUserId(), items); err != nil {
		return nil, err
	}
	return &cartv1.AddItemResp{}, nil
}

func (h *CartHandler) DeleteItem(ctx context.Context, req *cartv1.DeleteItemReq) (*cartv1.DeleteItemResp, error) {
	if err := h.CartService.DeleteItems(ctx, req.GetUserId(), req.GetSkuIds()); err != nil {
		return nil, err
	}
	return &cartv1.DeleteItemResp{}, nil
}

func (h *CartHandler) GetCart(ctx context.Context, req *cartv1.GetCartReq) (*cartv1.GetCartResp, error) {
	cart, err := h.CartService.GetCart(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	items := make([]*cartv1.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, &cartv1.CartItem{
			ProductId: item.ProductID,
			SkuId:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}

	return &cartv1.GetCartResp{
		Cart: &cartv1.Cart{
			UserId: cart.UserID,
			Items:  items,
		},
	}, nil
}

func (h *CartHandler) EmptyCart(ctx context.Context, req *cartv1.EmptyCartReq) (*cartv1.EmptyCartResp, error) {
	err := h.CartService.EmptyCart(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &cartv1.EmptyCartResp{}, nil
}

func (h *CartHandler) ChangeQty(ctx context.Context, req *cartv1.ChangeQtyReq) (*cartv1.ChangeQtyResp, error) {
	item := domain.CartItem{
		ProductID: req.GetItem().GetProductId(),
		SKUID:     req.GetItem().GetSkuId(),
		Quantity:  req.GetItem().GetQuantity(),
	}
	err := h.CartService.ChangeQty(ctx, req.GetUserId(), item)
	if err != nil {
		return nil, err
	}
	return &cartv1.ChangeQtyResp{}, nil
}

func (h *CartHandler) IncrementQty(ctx context.Context, req *cartv1.IncrementQtyReq) (*cartv1.IncrementQtyResp, error) {
	newQty, err := h.CartService.IncrementQty(ctx, req.GetUserId(), req.GetSkuId())
	if err != nil {
		return nil, err
	}
	return &cartv1.IncrementQtyResp{
		NewQuantity: newQty,
	}, nil
}

func (h *CartHandler) DecrementQty(ctx context.Context, req *cartv1.DecrementQtyReq) (*cartv1.DecrementQtyResp, error) {
	newQty, err := h.CartService.DecrementQty(ctx, req.GetUserId(), req.GetSkuId())
	if err != nil {
		return nil, err
	}
	return &cartv1.DecrementQtyResp{
		NewQuantity: newQty,
	}, nil
}
