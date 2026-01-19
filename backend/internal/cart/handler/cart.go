package handler

import (
	"context"
	"fmt"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
)

type CartHandler struct {
	CartService cartservice.CartService
}

func (h *CartHandler) AddItem(ctx context.Context, req *cartv1.AddItemReq) (res *cartv1.AddItemResp, err error) {
	//TODO implement me
	panic("implement me")
}

func (h *CartHandler) DeleteItem(ctx context.Context, req *cartv1.DeleteItemReq) (res *cartv1.DeleteItemResp, err error) {
	//TODO implement me
	panic("implement me")
}

func (h *CartHandler) GetCart(ctx context.Context, req *cartv1.GetCartReq) (res *cartv1.GetCartResp, err error) {
	//TODO implement me
	panic("implement me")
}

func (h *CartHandler) EmptyCart(ctx context.Context, req *cartv1.EmptyCartReq) (res *cartv1.EmptyCartResp, err error) {
	//TODO implement me
	panic("implement me")
}

func (h *CartHandler) ChangeQty(ctx context.Context, req *cartv1.ChangeQtyReq) (res *cartv1.ChangeQtyResp, err error) {
	//TODO implement me
	panic("implement me")
}
