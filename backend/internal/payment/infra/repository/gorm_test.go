package repository

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/stretchr/testify/require"
)

func TestToDBPaymentIncludesTxnID(t *testing.T) {
	dbPayment := toDBPayment(domain.Payment{
		BizTradeNo: "12345",
		Status:     domain.PaymentStatusSuccess,
		TxnID:      "wx_txn_1",
	})

	require.True(t, dbPayment.TxnID.Valid)
	require.Equal(t, "wx_txn_1", dbPayment.TxnID.String)
}

func TestNextPaymentStatusSuccessCannotBeOverwrittenByFailure(t *testing.T) {
	next, changed := nextPaymentStatus(domain.PaymentStatusSuccess, domain.PaymentStatusFailed)

	require.False(t, changed)
	require.Equal(t, domain.PaymentStatusSuccess, next)
}

func TestNextPaymentStatusFailureCanBePromotedToSuccess(t *testing.T) {
	next, changed := nextPaymentStatus(domain.PaymentStatusFailed, domain.PaymentStatusSuccess)

	require.True(t, changed)
	require.Equal(t, domain.PaymentStatusSuccess, next)
}
