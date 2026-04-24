package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	Remark         string
	ExpectedAmount int64
}

type PlaceOrderOutput struct {
	OrderID     int64
	PaymentURL  string
	TotalAmount int64
}

const (
	orderKindDirectBuy = "DIRECT_BUY"
	orderKindCart      = "CART"
	orderKindSeckill   = "SECKILL"
)

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, input PlaceOrderInput) (*PlaceOrderOutput, error) {
	if err := uc.validate(input); err != nil {
		return nil, err
	}
	orderKind, err := normalizeOrderKind(input.OrderKind)
	if err != nil {
		return nil, err
	}

	productResp, err := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: extractProductIDs(input.Items)})
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}

	lines, unavailable := buildOrderLines(input.Items, productResp.Product)
	if len(unavailable) > 0 {
		items := make([]domain.UnavailableItem, len(unavailable))
		for i, l := range unavailable {
			items[i] = domain.UnavailableItem{
				ProductID: l.ProductID,
				SKUID:     l.SKUID,
				Name:      l.Name,
				Reason:    l.UnavailReason,
			}
		}
		return nil, &domain.UnavailableItemsError{Items: items}
	}

	var couponDiscount int64
	if len(input.CouponIDs) > 0 {
		couponResp, err := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
			UserId: input.UserID,
			Items:  toCouponOrderItems(lines),
		})
		if err != nil {
			return nil, fmt.Errorf("list available coupons: %w", err)
		}
		coupons := toCouponOptions(couponResp.Coupons, lines)
		couponDiscount = sumSelectedCouponDiscount(coupons, input.CouponIDs)
	}

	price := domain.CalculatePrice(lines, couponDiscount)
	if err := domain.ValidatePriceChange(input.ExpectedAmount, price.TotalAmount); err != nil {
		return nil, err
	}

	orderID := uc.idGen.GenerateOrderID()
	commitResp, err := uc.inventoryClient.CommitStock(ctx, &inventoryv1.CommitStockReq{
		OperationId: operationID(orderID, "commit"),
		Items:       toInventoryStockItems(input.Items),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInsufficientStock, err)
	}
	if commitResp != nil && commitResp.GetStatusCode() != 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrInsufficientStock, commitResp.GetStatusMsg())
	}

	if len(input.CouponIDs) > 0 {
		couponReserveResp, err := uc.couponClient.ReserveCoupon(ctx, &couponv1.ReserveCouponReq{
			UserId:        input.UserID,
			UserCouponIds: input.CouponIDs,
			OrderId:       orderID,
			Items:         toCouponOrderItems(lines),
		})
		if err != nil {
			uc.restoreCommittedStock(ctx, orderID)
			uc.releaseReservedCoupons(ctx, orderID)
			return nil, fmt.Errorf("reserve coupons: %w", err)
		}
		if !couponReserveResp.GetSuccess() {
			uc.restoreCommittedStock(ctx, orderID)
			return nil, toCouponUnavailableError(couponReserveResp.GetFailures())
		}
	}

	if _, err := uc.orderClient.CreateOrder(ctx, &orderv1.CreateOrderReq{
		OrderId:       orderID,
		UserId:        input.UserID,
		Currency:      input.Currency,
		Address:       toOrderAddress(input.Address),
		Remark:        input.Remark,
		OrderKind:     orderKind,
		PayableAmount: price.TotalAmount,
		Items:         toOrderItems(lines, input.Currency),
	}); err != nil {
		uc.restoreCommittedStock(ctx, orderID)
		uc.releaseReservedCoupons(ctx, orderID)
		return nil, fmt.Errorf("%w: %v", domain.ErrOrderCreateFailed, err)
	}

	payResp, err := uc.paymentClient.NativePrepay(ctx, &paymentv1.NativePrePayRequest{
		Amt: &paymentv1.Amount{
			Total:    price.TotalAmount,
			Currency: input.Currency,
		},
		BizTradeNo:  fmt.Sprintf("%d", orderID),
		Description: fmt.Sprintf("order %d", orderID),
	})
	if err != nil {
		uc.logger.Warn("create initial payment session failed",
			logger.Int64("orderID", orderID),
			logger.Error(err),
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
		return errors.New("invalid user_id")
	}
	if len(input.Items) == 0 {
		return domain.ErrInvalidInput
	}
	if err := validateCheckoutItems(input.Items); err != nil {
		return err
	}
	if input.Address.ReceiverName == "" || input.Address.Phone == "" {
		return errors.New("address receiver_name and phone are required")
	}
	if input.PaymentMethod == "" {
		return errors.New("payment_method is required")
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

func normalizeOrderKind(orderKind string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(orderKind)) {
	case "", orderKindDirectBuy:
		return orderKindDirectBuy, nil
	case orderKindCart:
		return orderKindCart, nil
	case orderKindSeckill:
		return "", errors.New("seckill orders must use seckill submit flow")
	default:
		return "", fmt.Errorf("unsupported order_kind: %s", orderKind)
	}
}
