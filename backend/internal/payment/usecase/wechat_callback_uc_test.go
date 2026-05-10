package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestPayCallbackSuccessRequiresTxnID(t *testing.T) {
	uc := NewPayCallbackUC(nil, nil, nil, logger.NewNopLogger())

	err := uc.Execute(context.Background(), CallbackCmd{
		TradeState:    "SUCCESS",
		OutTradeNo:    "12345",
		TransactionId: " ",
	})

	require.True(t, errors.Is(err, domain.ErrPaymentTxnIDRequired))
}

func TestPayCallbackAlipaySuccessRequiresTradeNo(t *testing.T) {
	uc := NewPayCallbackUC(nil, nil, nil, logger.NewNopLogger())

	err := uc.Execute(context.Background(), CallbackCmd{
		TradeState:    "TRADE_SUCCESS",
		OutTradeNo:    "12345",
		TransactionId: "",
	})

	require.True(t, errors.Is(err, domain.ErrPaymentTxnIDRequired))
}
