package test

import (
	"context"
	"errors"
	"fmt"
	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"testing"

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
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/require"
)

func TestPlaceOrderReleasesCouponWhenCreateOrderFails(t *testing.T) {
	productClient := &stubProductClient{
		getProductsResp: &productv1.GetProductsResp{
			Product: []*productv1.Product{{
				Id:       1,
				SkuId:    101,
				Name:     "test product",
				Price:    100,
				Currency: "CNY",
				InStock:  true,
			}},
		},
	}
	inventoryClient := &stubInventoryClient{
		commitResp: &inventoryv1.InventoryOpResp{StatusCode: 0},
		refundResp: &inventoryv1.InventoryOpResp{StatusCode: 0},
	}
	couponClient := &stubCouponClient{
		listAvailableResp: &couponv1.ListAvailableCouponsResp{
			Coupons: []*couponv1.UserCoupon{{
				Id: 9,
				Template: &couponv1.CouponTemplate{
					Type:          couponv1.CouponType_COUPON_TYPE_FIXED,
					DiscountValue: 10,
				},
			}},
		},
		reserveResp: &couponv1.ReserveCouponResp{Success: true},
		releaseResp: &couponv1.ReleaseCouponResp{Success: true},
	}
	orderClient := &stubOrderClient{
		createOrderErr: errors.New("create failed"),
	}
	paymentClient := &stubPaymentClient{}

	uc := usecase.NewPlaceOrderUseCase(
		productClient,
		inventoryClient,
		couponClient,
		orderClient,
		paymentClient,
		staticOrderIDGenerator(12345),
		logger.NewNopLogger(),
	)

	_, err := uc.Execute(context.Background(), validPlaceOrderInput(90, []int64{9}))
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrOrderCreateFailed)
	require.Equal(t, int64(12345), refundOperationOrderID(inventoryClient.refundReq))
	require.Equal(t, int64(12345), couponClient.releaseReq.GetOrderId())
}

func TestPlaceOrderKeepsOrderWhenPrepayFails(t *testing.T) {
	productClient := &stubProductClient{
		getProductsResp: &productv1.GetProductsResp{
			Product: []*productv1.Product{{
				Id:       1,
				SkuId:    101,
				Name:     "test product",
				Price:    100,
				Currency: "CNY",
				InStock:  true,
			}},
		},
	}
	inventoryClient := &stubInventoryClient{
		commitResp: &inventoryv1.InventoryOpResp{StatusCode: 0},
	}
	orderClient := &stubOrderClient{
		createOrderResp:  &orderv1.CreateOrderResp{OrderId: 12345},
		changeStatusResp: &orderv1.ChangeOrderStatusResp{},
	}
	paymentClient := &stubPaymentClient{
		nativePrepayErr: errors.New("prepay failed"),
	}

	uc := usecase.NewPlaceOrderUseCase(
		productClient,
		inventoryClient,
		&stubCouponClient{},
		orderClient,
		paymentClient,
		staticOrderIDGenerator(12345),
		logger.NewNopLogger(),
	)

	output, err := uc.Execute(context.Background(), validPlaceOrderInput(100, nil))
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, int64(12345), output.OrderID)
	require.Empty(t, output.PaymentURL)
	require.Nil(t, orderClient.changeStatusReq)
}

func TestPlaceOrderPassesDiscountedPayableAmountToOrder(t *testing.T) {
	productClient := &stubProductClient{
		getProductsResp: &productv1.GetProductsResp{
			Product: []*productv1.Product{{
				Id:       1,
				SkuId:    101,
				Name:     "test product",
				Price:    100,
				Currency: "CNY",
				InStock:  true,
			}},
		},
	}
	inventoryClient := &stubInventoryClient{
		commitResp: &inventoryv1.InventoryOpResp{StatusCode: 0},
	}
	couponClient := &stubCouponClient{
		listAvailableResp: &couponv1.ListAvailableCouponsResp{
			Coupons: []*couponv1.UserCoupon{{
				Id: 9,
				Template: &couponv1.CouponTemplate{
					Type:          couponv1.CouponType_COUPON_TYPE_FIXED,
					DiscountValue: 10,
				},
			}},
		},
		reserveResp: &couponv1.ReserveCouponResp{Success: true},
	}
	orderClient := &stubOrderClient{
		createOrderResp: &orderv1.CreateOrderResp{OrderId: 12345},
	}
	paymentClient := &stubPaymentClient{
		nativePrepayResp: &paymentv1.NativePrePayResponse{CodeUrl: "wechat://pay"},
	}

	uc := usecase.NewPlaceOrderUseCase(
		productClient,
		inventoryClient,
		couponClient,
		orderClient,
		paymentClient,
		staticOrderIDGenerator(12345),
		logger.NewNopLogger(),
	)

	output, err := uc.Execute(context.Background(), validPlaceOrderInput(90, []int64{9}))
	require.NoError(t, err)
	require.NotNil(t, output)
	require.NotNil(t, orderClient.createOrderReq)
	require.Equal(t, int64(90), orderClient.createOrderReq.GetPayableAmount())
	require.Len(t, orderClient.createOrderReq.GetItems(), 1)
	require.Equal(t, int64(101), orderClient.createOrderReq.GetItems()[0].GetSkuId())
}

func TestPlaceOrderReturnsInsufficientStockOnlyForStockFailure(t *testing.T) {
	productClient := &stubProductClient{
		getProductsResp: &productv1.GetProductsResp{
			Product: []*productv1.Product{{
				Id:       1,
				SkuId:    101,
				Name:     "test product",
				Price:    100,
				Currency: "CNY",
				InStock:  true,
			}},
		},
	}
	inventoryClient := &stubInventoryClient{
		commitResp: &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: "库存不足"},
	}

	uc := usecase.NewPlaceOrderUseCase(
		productClient,
		inventoryClient,
		&stubCouponClient{},
		&stubOrderClient{},
		&stubPaymentClient{},
		staticOrderIDGenerator(12345),
		logger.NewNopLogger(),
	)

	_, err := uc.Execute(context.Background(), validPlaceOrderInput(100, nil))
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInsufficientStock)
}

func TestPlaceOrderDoesNotMaskInventoryTimeoutAsStockFailure(t *testing.T) {
	productClient := &stubProductClient{
		getProductsResp: &productv1.GetProductsResp{
			Product: []*productv1.Product{{
				Id:       1,
				SkuId:    101,
				Name:     "test product",
				Price:    100,
				Currency: "CNY",
				InStock:  true,
			}},
		},
	}
	inventoryClient := &stubInventoryClient{
		commitResp: &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: "commit stock failed: context deadline exceeded"},
	}

	uc := usecase.NewPlaceOrderUseCase(
		productClient,
		inventoryClient,
		&stubCouponClient{},
		&stubOrderClient{},
		&stubPaymentClient{},
		staticOrderIDGenerator(12345),
		logger.NewNopLogger(),
	)

	_, err := uc.Execute(context.Background(), validPlaceOrderInput(100, nil))
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrInsufficientStock)
	require.Contains(t, err.Error(), "commit stock failed")
}

type staticOrderIDGenerator int64

func (g staticOrderIDGenerator) GenerateOrderID() int64 {
	return int64(g)
}

func validPlaceOrderInput(expectedAmount int64, couponIDs []int64) usecase.PlaceOrderInput {
	return usecase.PlaceOrderInput{
		UserID: 1,
		Items: []domain.CheckoutItem{{
			ProductID: 1,
			SKUID:     101,
			Quantity:  1,
		}},
		CouponIDs:      couponIDs,
		PaymentMethod:  "WECHAT_NATIVE",
		Currency:       "CNY",
		ExpectedAmount: expectedAmount,
		Address: domain.Address{
			ReceiverName: "buyer",
			Phone:        "13800138000",
			Province:     "Shanghai",
			City:         "Shanghai",
			District:     "Pudong",
			Street:       "test street",
			ZipCode:      "200000",
		},
	}
}

type stubProductClient struct {
	getProductsResp      *productv1.GetProductsResp
	getProductsErr       error
	getProductQuotesResp *productv1.GetProductQuotesResp
	getProductQuotesErr  error
}

func (s *stubProductClient) ListProducts(context.Context, *productv1.ListProductsReq, ...callopt.Option) (*productv1.ListProductsResp, error) {
	panic("unexpected call")
}
func (s *stubProductClient) GetProducts(context.Context, *productv1.GetProductsReq, ...callopt.Option) (*productv1.GetProductsResp, error) {
	return s.getProductsResp, s.getProductsErr
}
func (s *stubProductClient) GetProductQuotes(context.Context, *productv1.GetProductQuotesReq, ...callopt.Option) (*productv1.GetProductQuotesResp, error) {
	if s.getProductQuotesResp != nil || s.getProductQuotesErr != nil {
		return s.getProductQuotesResp, s.getProductQuotesErr
	}
	if s.getProductsResp == nil {
		return nil, nil
	}
	quotes := make([]*productv1.ProductQuote, 0, len(s.getProductsResp.GetProduct()))
	for _, p := range s.getProductsResp.GetProduct() {
		if p == nil {
			continue
		}
		quotes = append(quotes, &productv1.ProductQuote{
			ProductId: p.GetId(),
			SkuId:     p.GetSkuId(),
			Price:     p.GetPrice(),
			Currency:  p.GetCurrency(),
			InStock:   p.GetInStock(),
		})
	}
	return &productv1.GetProductQuotesResp{ProductQuotes: quotes}, nil
}
func (s *stubProductClient) CreateProduct(context.Context, *productv1.CreateProductReq, ...callopt.Option) (*productv1.CreateProductResp, error) {
	panic("unexpected call")
}
func (s *stubProductClient) UpdateProduct(context.Context, *productv1.UpdateProductReq, ...callopt.Option) (*productv1.UpdateProductResp, error) {
	panic("unexpected call")
}
func (s *stubProductClient) DeleteProduct(context.Context, *productv1.DeleteProductReq, ...callopt.Option) (*productv1.DeleteProductResp, error) {
	panic("unexpected call")
}

var _ productservice.Client = (*stubProductClient)(nil)

type stubInventoryClient struct {
	commitResp *inventoryv1.InventoryOpResp
	commitErr  error
	refundResp *inventoryv1.InventoryOpResp
	refundErr  error
	refundReq  *inventoryv1.RefundStockReq
}

func (s *stubInventoryClient) GetInventory(context.Context, *inventoryv1.GetInventoryReq, ...callopt.Option) (*inventoryv1.GetInventoryResp, error) {
	panic("unexpected call")
}
func (s *stubInventoryClient) BatchGetInventory(context.Context, *inventoryv1.BatchGetInventoryReq, ...callopt.Option) (*inventoryv1.BatchGetInventoryResp, error) {
	panic("unexpected call")
}
func (s *stubInventoryClient) CommitStock(context.Context, *inventoryv1.CommitStockReq, ...callopt.Option) (*inventoryv1.InventoryOpResp, error) {
	return s.commitResp, s.commitErr
}
func (s *stubInventoryClient) RefundStock(_ context.Context, req *inventoryv1.RefundStockReq, _ ...callopt.Option) (*inventoryv1.InventoryOpResp, error) {
	s.refundReq = req
	return s.refundResp, s.refundErr
}
func (s *stubInventoryClient) AdjustStock(context.Context, *inventoryv1.AdjustStockReq, ...callopt.Option) (*inventoryv1.InventoryOpResp, error) {
	panic("unexpected call")
}

var _ inventoryservice.Client = (*stubInventoryClient)(nil)

type stubCouponClient struct {
	listAvailableResp *couponv1.ListAvailableCouponsResp
	listAvailableErr  error
	reserveResp       *couponv1.ReserveCouponResp
	reserveErr        error
	releaseResp       *couponv1.ReleaseCouponResp
	releaseErr        error
	releaseReq        *couponv1.ReleaseCouponReq
}

func (s *stubCouponClient) ListUserCoupons(context.Context, *couponv1.ListUserCouponsReq, ...callopt.Option) (*couponv1.ListUserCouponsResp, error) {
	panic("unexpected call")
}
func (s *stubCouponClient) ListAvailableCoupons(context.Context, *couponv1.ListAvailableCouponsReq, ...callopt.Option) (*couponv1.ListAvailableCouponsResp, error) {
	return s.listAvailableResp, s.listAvailableErr
}
func (s *stubCouponClient) ReserveCoupon(context.Context, *couponv1.ReserveCouponReq, ...callopt.Option) (*couponv1.ReserveCouponResp, error) {
	return s.reserveResp, s.reserveErr
}
func (s *stubCouponClient) CommitCoupon(context.Context, *couponv1.CommitCouponReq, ...callopt.Option) (*couponv1.CommitCouponResp, error) {
	panic("unexpected call")
}
func (s *stubCouponClient) ReleaseCoupon(_ context.Context, req *couponv1.ReleaseCouponReq, _ ...callopt.Option) (*couponv1.ReleaseCouponResp, error) {
	s.releaseReq = req
	return s.releaseResp, s.releaseErr
}
func (s *stubCouponClient) RefundCoupon(context.Context, *couponv1.RefundCouponReq, ...callopt.Option) (*couponv1.RefundCouponResp, error) {
	panic("unexpected call")
}
func (s *stubCouponClient) CreateCouponTemplate(context.Context, *couponv1.CreateCouponTemplateReq, ...callopt.Option) (*couponv1.CreateCouponTemplateResp, error) {
	panic("unexpected call")
}
func (s *stubCouponClient) IssueCoupon(context.Context, *couponv1.IssueCouponReq, ...callopt.Option) (*couponv1.IssueCouponResp, error) {
	panic("unexpected call")
}

var _ couponservice.Client = (*stubCouponClient)(nil)

type stubOrderClient struct {
	createOrderResp  *orderv1.CreateOrderResp
	createOrderErr   error
	createOrderReq   *orderv1.CreateOrderReq
	changeStatusResp *orderv1.ChangeOrderStatusResp
	changeStatusErr  error
	changeStatusReq  *orderv1.ChangeOrderStatusReq
}

func (s *stubOrderClient) CreateOrder(_ context.Context, req *orderv1.CreateOrderReq, _ ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	s.createOrderReq = req
	return s.createOrderResp, s.createOrderErr
}
func (s *stubOrderClient) ChangeOrderStatus(_ context.Context, req *orderv1.ChangeOrderStatusReq, _ ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	s.changeStatusReq = req
	return s.changeStatusResp, s.changeStatusErr
}
func (s *stubOrderClient) GetOrder(context.Context, *orderv1.GetOrderReq, ...callopt.Option) (*orderv1.GetOrderResp, error) {
	panic("unexpected call")
}
func (s *stubOrderClient) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	panic("unexpected call")
}

var _ orderservice.Client = (*stubOrderClient)(nil)

type stubPaymentClient struct {
	nativePrepayResp *paymentv1.NativePrePayResponse
	nativePrepayErr  error
}

func (s *stubPaymentClient) NativePrepay(context.Context, *paymentv1.NativePrePayRequest, ...callopt.Option) (*paymentv1.NativePrePayResponse, error) {
	return s.nativePrepayResp, s.nativePrepayErr
}
func (s *stubPaymentClient) GetPayment(context.Context, *paymentv1.GetPaymentRequest, ...callopt.Option) (*paymentv1.GetPaymentResponse, error) {
	panic("unexpected call")
}
func (s *stubPaymentClient) ConfirmPayment(context.Context, *paymentv1.ConfirmPaymentRequest, ...callopt.Option) (*paymentv1.ConfirmPaymentResponse, error) {
	panic("unexpected call")
}

var _ paymentservice.Client = (*stubPaymentClient)(nil)

func refundOperationOrderID(req *inventoryv1.RefundStockReq) int64 {
	var orderID int64
	_, _ = fmt.Sscanf(req.GetOperationId(), "order_%d_refund", &orderID)
	return orderID
}
