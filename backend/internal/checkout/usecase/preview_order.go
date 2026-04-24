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

// 缁撶畻棰勮鐢ㄤ緥
// ai tool 閭ｈ竟鍙兘鍒拌繖涓€姝ワ紝鏈€鍚庤鐢ㄦ埛鐪嬭繖涓晫闈紝閫夋嫨浼樻儬鍒革紝纭畾鎵€鏈変俊鎭紝鐐瑰嚮鏀粯

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
	// 鍙傛暟鏍￠獙
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if len(input.Items) == 0 {
		return nil, domain.ErrInvalidInput
	}
	if err := validateCheckoutItems(input.Items); err != nil {
		return nil, err
	}

	// 1. 鏌ヨ鍟嗗搧
	resp, err := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: extractProductIDs(input.Items)})
	if err != nil {
		return nil, fmt.Errorf("get products: %w", err)
	}

	// 2. 鏋勫缓璁㈠崟琛岋細鐩存帴鍒嗘祦涓哄彲璐拱 / 澶辨晥涓ょ粍
	available, unavailable := buildOrderLines(input.Items, resp.Product)

	// 3. 浠呯敤鍙喘涔拌鏌ヨ浼樻儬鍒?
	couponResp, err := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
		UserId: input.UserID,
		Items:  toCouponOrderItems(available),
	})
	if err != nil {
		return nil, fmt.Errorf("list available coupons: %w", err)
	}
	coupons := toCouponOptions(couponResp.Coupons, available)

	// Preview 鎶婂叏閮ㄨ锛堝惈澶辨晥锛夎繑鍥炵粰鍓嶇灞曠ず
	return &PreviewOrderOutput{
		Lines:            append(available, unavailable...),
		AvailableCoupons: coupons,
	}, nil
}
