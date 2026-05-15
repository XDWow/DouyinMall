package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1/couponservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
)

// PreviewOrderUseCase prepares checkout preview data before the user places an order.
// The response is meant for the confirmation page where the user reviews products
// and picks coupons before submitting payment.
type PreviewOrderUseCase struct {
	productClient productservice.Client
	couponClient  couponservice.Client
}

func NewPreviewOrderUseCase(
	productClient productservice.Client,
	couponClient couponservice.Client,
) *PreviewOrderUseCase {
	return &PreviewOrderUseCase{
		productClient: productClient,
		couponClient:  couponClient,
	}
}

type PreviewOrderInput struct {
	UserID int64
	Items  []domain.CheckoutItem
}

type PreviewOrderOutput struct {
	Lines            []domain.OrderLine
	AvailableCoupons []domain.CouponOption
}

func (uc *PreviewOrderUseCase) Execute(ctx context.Context, input PreviewOrderInput) (*PreviewOrderOutput, error) {
	// Validate request parameters.
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if len(input.Items) == 0 {
		return nil, domain.ErrInvalidInput
	}
	if err := validateCheckoutItems(input.Items); err != nil {
		return nil, err
	}

	// 1. Query product snapshots for all requested items.
	resp, err := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Items: extractProductQueries(input.Items)})
	if err != nil {
		return nil, fmt.Errorf("get products: %w", err)
	}

	// 2. Split items into available and unavailable order lines.
	available, unavailable := buildOrderLines(input.Items, resp.Product)

	// 3. Only available items participate in coupon evaluation.
	couponResp, err := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
		UserId: input.UserID,
		Items:  toCouponOrderItems(available),
	})
	if err != nil {
		return nil, fmt.Errorf("list available coupons: %w", err)
	}
	coupons := toCouponOptions(couponResp.Coupons, available)

	// Return all lines, including unavailable ones, so the UI can explain what changed.
	return &PreviewOrderOutput{
		Lines:            append(available, unavailable...),
		AvailableCoupons: coupons,
	}, nil
}
