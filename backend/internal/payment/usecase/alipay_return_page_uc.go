package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

type AlipayReturnPageUC struct {
	orderClient orderservice.Client
}

type AlipayReturnPageCmd struct {
	OrderID    int64
	OutTradeNo string
}

type AlipayReturnPageResult struct {
	OrderID     int64
	OutTradeNo  string
	OrderStatus string
}

func NewAlipayReturnPageUC(orderClient orderservice.Client) *AlipayReturnPageUC {
	return &AlipayReturnPageUC{orderClient: orderClient}
}

func (uc *AlipayReturnPageUC) Execute(ctx context.Context, cmd AlipayReturnPageCmd) (*AlipayReturnPageResult, error) {
	orderID := cmd.OrderID
	if orderID <= 0 && strings.TrimSpace(cmd.OutTradeNo) != "" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(cmd.OutTradeNo), 10, 64)
		if err == nil {
			orderID = parsed
		}
	}

	result := &AlipayReturnPageResult{
		OrderID:    orderID,
		OutTradeNo: strings.TrimSpace(cmd.OutTradeNo),
	}
	if orderID <= 0 || uc.orderClient == nil {
		return result, nil
	}

	resp, err := uc.orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: orderID})
	if err != nil {
		return nil, fmt.Errorf("query order failed: %w", err)
	}
	if resp.GetOrder() != nil {
		result.OrderStatus = resp.GetOrder().GetOrderStatus().String()
		if result.OutTradeNo == "" {
			result.OutTradeNo = strconv.FormatInt(resp.GetOrder().GetOrderId(), 10)
		}
	}
	return result, nil
}
