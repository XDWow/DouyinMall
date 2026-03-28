package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1/couponservice"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
)

// 普通下单
type PlaceOrderUseCase struct {
	productClient   productservice.Client
	inventoryClient inventoryservice.Client
	couponClient    couponservice.Client
	orderClient     orderservice.Client
	paymentClient   paymentservice.Client

	idGen  IDGenerator
	logger logger.LoggerV1
}

func NewPlaceOrderUseCase(
	productClient productservice.Client,
	inventoryClient inventoryservice.Client,
	couponClient couponservice.Client,
	orderClient orderservice.Client,
	paymentClient paymentservice.Client,
	idGen IDGenerator,
	logger logger.LoggerV1,
) *PlaceOrderUseCase {
	return &PlaceOrderUseCase{
		productClient:   productClient,
		inventoryClient: inventoryClient,
		couponClient:    couponClient,
		orderClient:     orderClient,
		paymentClient:   paymentClient,
		idGen:           idGen,
		logger:          logger,
	}
}

type PlaceOrderInput struct {
	UserID         int64
	Items          []domain.CheckoutItem
	CouponIDs      []int64
	Address        domain.Address
	PaymentMethod  string
	Currency       string
	OrderKind      string
	Remark         string // 订单备注
	ExpectedAmount int64  // preview 中看到的金额，真正支付的时候会兜底再算一次，如果跟之前用户看到的不一样，就返回
}

type PlaceOrderOutput struct {
	OrderID     int64
	PaymentURL  string
	TotalAmount int64
}

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, input PlaceOrderInput) (output *PlaceOrderOutput, err error) {
	if vErr := uc.validate(input); vErr != nil {
		return nil, vErr
	}

	productResp, queryErr := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: extractProductIDs(input.Items)})
	if queryErr != nil {
		return nil, fmt.Errorf("query products: %w", queryErr)
	}

	lines, unavailable := buildOrderLines(input.Items, productResp.Product)
	// 有的订单项失效了，必须返回，用户可以选择重新下单
	if len(unavailable) > 0 {
		items := make([]domain.UnavailableItem, len(unavailable))
		for i, l := range unavailable {
			items[i] = domain.UnavailableItem{ProductID: l.ProductID, Name: l.Name, Reason: l.UnavailReason}
		}
		return nil, &domain.UnavailableItemsError{Items: items}
	}

	// 查一下选择的优惠券是否还在可使用列表中
	var couponDiscount int64
	if len(input.CouponIDs) > 0 {
		couponResp, couponErr := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
			UserId: input.UserID,
			Items:  toCouponOrderItems(lines),
		})
		if couponErr != nil {
			return nil, fmt.Errorf("查询优惠券出错: %w", couponErr)
		}
		coupons := toCouponOptions(couponResp.Coupons, lines)
		couponDiscount = sumSelectedCouponDiscount(coupons, input.CouponIDs)
	}

	price := domain.CalculatePrice(lines, couponDiscount)
	if priceErr := domain.ValidatePriceChange(input.ExpectedAmount, price.TotalAmount); priceErr != nil {
		return nil, priceErr
	}
	// --- 商品、优惠券校验完成 ---

	orderID := uc.idGen.GenerateOrderID()
	commitResp, commitErr := uc.inventoryClient.CommitStock(ctx, &inventoryv1.CommitStockReq{
		OperationId: operationID(orderID, "commit"),
		Items:       toInventoryStockItems(input.Items),
	})
	if commitErr != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInsufficientStock, commitErr)
	}
	if commitResp != nil && commitResp.StatusCode != 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrInsufficientStock, commitResp.GetStatusMsg())
	}

	if len(input.CouponIDs) > 0 {
		couponReserveResp, couponErr := uc.couponClient.ReserveCoupon(ctx, &couponv1.ReserveCouponReq{
			UserId:        input.UserID,
			UserCouponIds: input.CouponIDs,
			OrderId:       orderID,
			Items:         toCouponOrderItems(lines),
		})
		if couponErr != nil { // 调用出错，不知道优惠券扣了还是没扣，也是失败，回退优惠券，避免锁住优惠券
			uc.restoreCommittedStock(ctx, orderID)
			uc.releaseReservedCoupons(ctx, orderID)
			return nil, fmt.Errorf("reserve coupons: %w", couponErr)
		}
		if !couponReserveResp.Success { // 明确是扣失败了
			uc.restoreCommittedStock(ctx, orderID)
			return nil, toCouponUnavailableError(couponReserveResp.Failures)
		}
	}

	if _, createErr := uc.orderClient.CreateOrder(ctx, &orderv1.CreateOrderReq{
		OrderId:       orderID,
		UserId:        input.UserID,
		Currency:      input.Currency,
		Address:       toOrderAddress(input.Address),
		Remark:        input.Remark,
		OrderKind:     normalizeOrderKind(input.OrderKind),
		PayableAmount: price.TotalAmount,
		Items:         toOrderItems(lines, input.Currency),
	}); createErr != nil {
		uc.restoreCommittedStock(ctx, orderID)
		uc.releaseReservedCoupons(ctx, orderID)
		return nil, fmt.Errorf("%w: %v", domain.ErrOrderCreateFailed, createErr)
	}

	payResp, payErr := uc.paymentClient.NativePrepay(ctx, &paymentv1.NativePrePayRequest{
		Amt: &paymentv1.Amount{
			Total:    price.TotalAmount,
			Currency: input.Currency,
		},
		BizTradeNo:  fmt.Sprintf("%d", orderID),
		Description: fmt.Sprintf("订单 %d 支付", orderID),
	})
	if payErr != nil { // 调用支付失败，不影响订单创建，打个日志
		uc.logger.Warn("create initial payment session failed",
			logger.Int64("orderID", orderID),
			logger.Error(payErr),
		)
		return &PlaceOrderOutput{
			OrderID:     orderID,
			TotalAmount: price.TotalAmount,
		}, nil
	}

	return &PlaceOrderOutput{
		OrderID:     orderID,
		PaymentURL:  payResp.CodeUrl,
		TotalAmount: price.TotalAmount,
	}, nil
}

func (uc *PlaceOrderUseCase) validate(input PlaceOrderInput) error {
	if input.UserID <= 0 {
		return errors.New("无效的用户id")
	}
	if len(input.Items) == 0 {
		return domain.ErrInvalidInput
	}
	if input.Address.ReceiverName == "" || input.Address.Phone == "" {
		return errors.New("地址补完整")
	}
	if input.PaymentMethod == "" {
		return errors.New("未选择支付方式")
	}
	if input.ExpectedAmount <= 0 {
		return errors.New("expected amount must be positive")
	}
	return nil
}

func (uc *PlaceOrderUseCase) restoreCommittedStock(ctx context.Context, orderID int64) {
	if _, err := uc.inventoryClient.RefundStock(ctx, &inventoryv1.RefundStockReq{
		OperationId: operationID(orderID, "refund"),
	}); err != nil {
		uc.logger.Error("restore committed stock failed",
			logger.Int64("orderID", orderID),
			logger.Error(err),
		)
	}
}

func (uc *PlaceOrderUseCase) releaseReservedCoupons(ctx context.Context, orderID int64) {
	if _, err := uc.couponClient.ReleaseCoupon(ctx, &couponv1.ReleaseCouponReq{
		OrderId: orderID,
	}); err != nil {
		uc.logger.Error("release reserved coupons failed",
			logger.Int64("orderID", orderID),
			logger.Error(err),
		)
	}
}

func normalizeOrderKind(orderKind string) string {
	if orderKind == "" {
		return "DIRECT_BUY"
	}
	return orderKind
}
