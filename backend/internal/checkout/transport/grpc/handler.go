package grpc

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	checkoutv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1"
)

type CheckoutHandler struct {
	previewUC *usecase.PreviewOrderUseCase
	placeUC   *usecase.PlaceOrderUseCase
	payUC     *usecase.PayOrderUseCase
}

func NewCheckoutHandler(
	previewUC *usecase.PreviewOrderUseCase,
	placeUC *usecase.PlaceOrderUseCase,
	payUC *usecase.PayOrderUseCase,
) *CheckoutHandler {
	return &CheckoutHandler{
		previewUC: previewUC,
		placeUC:   placeUC,
		payUC:     payUC,
	}
}

func (h *CheckoutHandler) PreviewOrder(ctx context.Context, req *checkoutv1.PreviewOrderReq) (*checkoutv1.PreviewOrderResp, error) {
	output, err := h.previewUC.Execute(ctx, usecase.PreviewOrderInput{
		UserID: req.GetUserId(),
		Items:  toCheckoutItems(req.GetItems()),
	})
	if err != nil {
		return nil, err
	}

	return &checkoutv1.PreviewOrderResp{
		Products:         toProductDetails(output.Lines),
		AvailableCoupons: toCouponItems(output.AvailableCoupons),
	}, nil
}

func (h *CheckoutHandler) PlaceOrder(ctx context.Context, req *checkoutv1.PlaceOrderReq) (*checkoutv1.PlaceOrderResp, error) {
	output, err := h.placeUC.Execute(ctx, usecase.PlaceOrderInput{
		UserID:         req.GetUserId(),
		Items:          toCheckoutItems(req.GetItems()),
		CouponIDs:      req.GetCouponIds(),
		Address:        toAddress(req.GetAddress()),
		PaymentMethod:  req.GetPaymentMethod(),
		Currency:       req.GetCurrency(),
		OrderKind:      req.GetOrderKind(),
		Remark:         req.GetRemark(),
		ExpectedAmount: req.GetExpectedAmount(),
	})
	if err != nil {
		return nil, err
	}

	return &checkoutv1.PlaceOrderResp{
		OrderId:     output.OrderID,
		PaymentUrl:  output.PaymentURL,
		TotalAmount: output.TotalAmount,
		ExpireAt:    time.Now().Add(30 * time.Minute).Unix(),
	}, nil
}

func (h *CheckoutHandler) PayOrder(ctx context.Context, req *checkoutv1.PayOrderReq) (*checkoutv1.PayOrderResp, error) {
	output, err := h.payUC.Execute(ctx, usecase.PayOrderInput{
		UserID:  req.GetUserId(),
		OrderID: req.GetOrderId(),
	})
	if err != nil {
		return nil, err
	}
	return &checkoutv1.PayOrderResp{
		OrderId:     output.OrderID,
		PaymentUrl:  output.PaymentURL,
		TotalAmount: output.TotalAmount,
		ExpireAt:    output.ExpireAt,
	}, nil
}

func toCheckoutItems(items []*checkoutv1.CheckoutItem) []domain.CheckoutItem {
	result := make([]domain.CheckoutItem, 0, len(items))
	for _, it := range items {
		result = append(result, domain.CheckoutItem{
			ProductID: it.GetProductId(),
			Quantity:  it.GetQuantity(),
		})
	}
	return result
}

func toAddress(a *checkoutv1.Address) domain.Address {
	if a == nil {
		return domain.Address{}
	}
	return domain.Address{
		ReceiverName: a.GetReceiverName(),
		Phone:        a.GetPhone(),
		Province:     a.GetProvince(),
		City:         a.GetCity(),
		District:     a.GetDistrict(),
		Street:       a.GetStreet(),
		ZipCode:      a.GetZipCode(),
	}
}

func toProductDetails(lines []domain.OrderLine) []*checkoutv1.ProductDetail {
	result := make([]*checkoutv1.ProductDetail, 0, len(lines))
	for _, l := range lines {
		result = append(result, &checkoutv1.ProductDetail{
			ProductId:         l.ProductID,
			Name:              l.Name,
			Picture:           l.Picture,
			Price:             l.Price,
			Currency:          l.Currency,
			Quantity:          l.Quantity,
			Subtotal:          l.Subtotal,
			Available:         l.Available,
			UnavailableReason: l.UnavailReason,
		})
	}
	return result
}

func toCouponItems(options []domain.CouponOption) []*checkoutv1.CouponItem {
	result := make([]*checkoutv1.CouponItem, 0, len(options))
	for _, c := range options {
		result = append(result, &checkoutv1.CouponItem{
			CouponId:         c.CouponID,
			Name:             c.Name,
			Description:      c.Description,
			DiscountAmount:   c.DiscountAmount,
			Usable:           c.Usable,
			UnusableReason:   c.UnusableReason,
			PerLineDiscounts: c.PerLineDiscounts,
		})
	}
	return result
}
