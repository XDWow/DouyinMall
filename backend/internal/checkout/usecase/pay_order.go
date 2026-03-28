package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
)

// 选择一个订单支付，是当时创建了订单，但没支付的情况
type PayOrderUseCase struct {
	orderClient   orderservice.Client
	paymentClient paymentservice.Client
}

func NewPayOrderUseCase(
	orderClient orderservice.Client,
	paymentClient paymentservice.Client,
) *PayOrderUseCase {
	return &PayOrderUseCase{
		orderClient:   orderClient,
		paymentClient: paymentClient,
	}
}

type PayOrderInput struct {
	UserID  int64
	OrderID int64
}

type PayOrderOutput struct {
	OrderID     int64
	PaymentURL  string
	TotalAmount int64
	ExpireAt    int64 // 订单取消还剩多少时间
}

func (uc *PayOrderUseCase) Execute(ctx context.Context, input PayOrderInput) (*PayOrderOutput, error) {
	if input.UserID <= 0 || input.OrderID <= 0 {
		return nil, domain.ErrInvalidInput
	}

	orderResp, err := uc.orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: input.OrderID})
	if err != nil {
		return nil, fmt.Errorf("query order: %w", err)
	}
	order := orderResp.GetOrder()
	if order == nil {
		return nil, domain.ErrOrderCreateFailed
	}
	if order.GetUserId() != input.UserID {
		return nil, domain.ErrOrderForbidden
	}
	if order.GetOrderStatus() != orderv1.OrderStatus_ORDER_STATUS_CREATED {
		return nil, domain.ErrOrderNotPayable
	}
	if order.GetExpireAt() > 0 && order.GetExpireAt() <= time.Now().Unix() {
		_, _ = uc.orderClient.ChangeOrderStatus(ctx, &orderv1.ChangeOrderStatusReq{
			OrderId: input.OrderID,
			Action:  orderv1.ChangeOrderAction_CHANGE_ORDER_ACTION_CANCEL,
		})
		return nil, domain.ErrOrderExpired
	}

	payResp, err := uc.paymentClient.NativePrepay(ctx, &paymentv1.NativePrePayRequest{
		Amt: &paymentv1.Amount{
			Total:    order.GetTotalAmount(),
			Currency: order.GetCurrency(),
		},
		BizTradeNo:  fmt.Sprintf("%d", input.OrderID),
		Description: fmt.Sprintf("order %d payment", input.OrderID),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPaymentCreateFailed, err)
	}

	return &PayOrderOutput{
		OrderID:     input.OrderID,
		PaymentURL:  payResp.GetCodeUrl(),
		TotalAmount: order.GetTotalAmount(),
		ExpireAt:    order.GetExpireAt(),
	}, nil
}
