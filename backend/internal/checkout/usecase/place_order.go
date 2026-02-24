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
	Remark         string
	ExpectedAmount int64 // 用户结算页看到的应付金额，用来判断价格是否变动，因为怕用户看到的金额，和最后支付金额不一样
}

type PlaceOrderOutput struct {
	OrderID     int64
	PaymentURL  string
	TotalAmount int64
}

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, input PlaceOrderInput) (output *PlaceOrderOutput, err error) {
	// 参数校验
	if vErr := uc.validate(input); vErr != nil {
		return nil, vErr
	}

	// 读阶段：要再读一下拿最新信息，之前 preview 读到的没有时效性
	productResp, queryErr := uc.productClient.GetProducts(ctx, &productv1.GetProductsReq{Id: extractProductIDs(input.Items)})
	if queryErr != nil {
		return nil, fmt.Errorf("query products: %w", queryErr)
	}
	available, unavailable := buildOrderLines(input.Items, productResp.Product)
	// 小概率事件：商品刚好下架了，要返回已经下架的商品，前端提示用户哪些商品已经下架
	if len(unavailable) > 0 {
		items := make([]domain.UnavailableItem, len(unavailable))
		for i, l := range unavailable {
			items[i] = domain.UnavailableItem{ProductID: l.ProductID, Name: l.Name, Reason: l.UnavailReason}
		}
		return nil, &domain.UnavailableItemsError{Items: items}
	}
	lines := available

	var couponDiscount int64
	if len(input.CouponIDs) > 0 {
		// 再次确认可用优惠券
		couponResp, couponErr := uc.couponClient.ListAvailableCoupons(ctx, &couponv1.ListAvailableCouponsReq{
			UserId: input.UserID,
			Items:  toCouponOrderItems(lines),
		})
		if couponErr != nil {
			return nil, fmt.Errorf("query coupons: %w", couponErr)
		}
		// 所有可用优惠券的情况，每个 Line 优惠多少钱，方便前端直接切换优惠券，自己计算价格
		coupons := toCouponOptions(couponResp.Coupons, lines)
		couponDiscount = sumSelectedCouponDiscount(coupons, input.CouponIDs)
	}

	price := domain.CalculatePrice(lines, couponDiscount)
	if priceErr := domain.ValidatePriceChange(input.ExpectedAmount, price.TotalAmount); priceErr != nil {
		return nil, priceErr
	}

	// 写阶段：依次预扣资源 → 创建订单 → 创建支付
	// checkout 是无状态编排层，无需主动补偿：预扣库存 TTL 自动过期，订单超时自动取消 → MQ 驱动下游释放资源

	// 预生成 OrderID（雪花ID，全局唯一，用于关联 inventory/coupon/order）
	orderID := uc.idGen.GenerateOrderID()

	// 1. 预扣库存
	reserveResp, reserveErr := uc.inventoryClient.ReserveStock(ctx, &inventoryv1.ReserveStockReq{
		OperationId: operationID(orderID, "reserve"),
		Items:       toInventoryStockItems(input.Items),
		ExpireTime:  1800, // 30分钟过期兜底
	})
	if reserveErr != nil {
		// 从 resp 取库存不足明细返回给前端
		if reserveResp != nil && len(reserveResp.InsufficientItems) > 0 {
			return nil, toInsufficientStockError(reserveResp.InsufficientItems, lines)
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrInsufficientStock, reserveErr)
	}

	// 2. 批量锁定优惠券
	if len(input.CouponIDs) > 0 {
		couponReserveResp, couponErr := uc.couponClient.ReserveCoupon(ctx, &couponv1.ReserveCouponReq{
			UserId:        input.UserID,
			UserCouponIds: input.CouponIDs,
			OrderId:       orderID,
			Items:         toCouponOrderItems(lines), // 优惠券那边用来计算优惠券可用性，锁定前的兜底确认
		})
		if couponErr != nil {
			return nil, fmt.Errorf("reserve coupons: %w", couponErr)
		}
		if !couponReserveResp.Success {
			return nil, toCouponUnavailableError(couponReserveResp.Failures)
		}
	}

	// 3. 资源预占成功，可以创建订单（传入预生成的 orderID）
	if _, createErr := uc.orderClient.CreateOrder(ctx, &orderv1.CreateOrderReq{
		OrderId:  orderID,
		UserId:   input.UserID,
		Currency: input.Currency,
		Address:  toOrderAddress(input.Address),
		Phone:    input.Address.Phone,
		Items:    toOrderItems(lines, input.Currency),
	}); createErr != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrOrderCreateFailed, createErr)
	}

	// 4. 创建支付单
	payResp, payErr := uc.paymentClient.NativePrepay(ctx, &paymentv1.NativePrePayRequest{
		Amt: &paymentv1.Amount{
			Total:    price.TotalAmount,
			Currency: input.Currency,
		},
		BizTradeNo:  fmt.Sprintf("%d", orderID),
		Description: fmt.Sprintf("订单 %d 支付", orderID),
	})
	if payErr != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPaymentCreateFailed, payErr)
	}

	return &PlaceOrderOutput{
		OrderID:     orderID,
		PaymentURL:  payResp.CodeUrl,
		TotalAmount: price.TotalAmount,
	}, nil
}

// 内部方法

func (uc *PlaceOrderUseCase) validate(input PlaceOrderInput) error {
	if input.UserID <= 0 {
		return errors.New("invalid user_id")
	}
	if len(input.Items) == 0 {
		return domain.ErrInvalidInput
	}
	if input.Address.ReceiverName == "" || input.Address.Phone == "" {
		return errors.New("address is incomplete")
	}
	if input.PaymentMethod == "" {
		return errors.New("payment method is required")
	}
	if input.ExpectedAmount <= 0 {
		return errors.New("expected amount must be positive")
	}
	return nil
}
