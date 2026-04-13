package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
)

type PaymentHandler struct {
	prePayUC  *usecase.NativePrePaymentUC
	getPayUC  *usecase.GetPaymentUC
	confirmUC *usecase.ConfirmPaymentUC
}

func NewPaymentHandler(
	prePayUC *usecase.NativePrePaymentUC,
	getPayUC *usecase.GetPaymentUC,
	confirmUC *usecase.ConfirmPaymentUC,
) *PaymentHandler {
	return &PaymentHandler{
		prePayUC:  prePayUC,
		getPayUC:  getPayUC,
		confirmUC: confirmUC,
	}
}

func (h *PaymentHandler) NativePrepay(ctx context.Context, req *paymentv1.NativePrePayRequest) (*paymentv1.NativePrePayResponse, error) {
	cmd := usecase.PrePaymentCmd{
		Amt: domain.Amount{
			Total:    req.GetAmt().GetTotal(),
			Currency: req.GetAmt().GetCurrency(),
		},
		BizTradeNo:  req.GetBizTradeNo(),
		Description: req.GetDescription(),
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	codeURL, err := h.prePayUC.Execute(callCtx, cmd)
	if err != nil {
		return nil, err
	}
	return &paymentv1.NativePrePayResponse{CodeUrl: codeURL}, nil
}

func (h *PaymentHandler) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	if req.GetBizTradeNo() == "" {
		return nil, errors.New("业务交易号不能为空")
	}

	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	pmt, err := h.getPayUC.Execute(callCtx, usecase.QueryPaymentCmd{BizTradeNo: req.GetBizTradeNo()})
	if err != nil {
		return nil, err
	}
	return &paymentv1.GetPaymentResponse{Status: toProtoPaymentStatus(pmt.Status)}, nil
}

func (h *PaymentHandler) ConfirmPayment(ctx context.Context, req *paymentv1.ConfirmPaymentRequest) (*paymentv1.ConfirmPaymentResponse, error) {
	if req.GetBizTradeNo() == "" {
		return nil, errors.New("业务交易号不能为空")
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pmt, err := h.confirmUC.Execute(callCtx, req.GetBizTradeNo())
	if err != nil {
		return nil, err
	}
	return &paymentv1.ConfirmPaymentResponse{
		Status: toProtoPaymentStatus(pmt.Status),
		TxnId:  pmt.TxnID,
	}, nil
}

func toProtoPaymentStatus(s domain.PaymentStatus) paymentv1.PaymentStatus {
	switch s {
	case domain.PaymentStatusInit:
		return paymentv1.PaymentStatus_PaymentStatusInit
	case domain.PaymentStatusSuccess:
		return paymentv1.PaymentStatus_PaymentStatusSuccess
	case domain.PaymentStatusFailed:
		return paymentv1.PaymentStatus_PaymentStatusFailed
	case domain.PaymentStatusRefund:
		return paymentv1.PaymentStatus_PaymentStatusRefund
	default:
		return paymentv1.PaymentStatus_PaymentStatusUnknown
	}
}
