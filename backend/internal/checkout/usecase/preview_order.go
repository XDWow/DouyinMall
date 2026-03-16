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

// 结算预览用例
// ai tool 那边只能到这一步，最后要用户看这个界面，选择优惠券，确定所有信息，点击支付

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
	// 参数校验
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if len(input.Items) == 0 {
		return nil, domain.ErrInvalidInput
	}

	// 1. 查询商品
	resp, err := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: extractProductIDs(input.Items)})
	if err != nil {
		return nil, fmt.Errorf("get products: %w", err)
	}

	// 2. 构建订单行：直接分流为可购买 / 失效两组
	available, unavailable := buildOrderLines(input.Items, resp.Product)

	// 3. 仅用可购买行查询优惠券
	couponResp, err := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
		UserId: input.UserID,
		Items:  toCouponOrderItems(available),
	})
	if err != nil {
		return nil, fmt.Errorf("list available coupons: %w", err)
	}
	coupons := toCouponOptions(couponResp.Coupons, available)

	// Preview 把全部行（含失效）返回给前端展示
	return &PreviewOrderOutput{
		Lines:            append(available, unavailable...),
		AvailableCoupons: coupons,
	}, nil
}
