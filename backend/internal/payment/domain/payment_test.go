package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentApplyProviderResultSuccessCannotBeOverwrittenByFailure(t *testing.T) {
	payment := Payment{Status: PaymentStatusSuccess, TxnID: "wx_txn_1"}

	transition := payment.ApplyProviderResult(Payment{Status: PaymentStatusFailed})

	require.False(t, transition.StatusChanged)
	require.False(t, transition.ShouldPersist)
	require.Equal(t, PaymentStatusSuccess, transition.Payment.Status)
	require.Equal(t, "wx_txn_1", transition.Payment.TxnID)
}

func TestPaymentApplyProviderResultFailureCanBePromotedToSuccess(t *testing.T) {
	payment := Payment{Status: PaymentStatusFailed}

	transition := payment.ApplyProviderResult(Payment{Status: PaymentStatusSuccess, TxnID: "wx_txn_1"})

	require.True(t, transition.StatusChanged)
	require.True(t, transition.ShouldPersist)
	require.Equal(t, PaymentStatusSuccess, transition.Payment.Status)
	require.Equal(t, "wx_txn_1", transition.Payment.TxnID)
}

func TestPaymentApplyProviderResultSuccessCanFillTxnIDWithoutStatusEvent(t *testing.T) {
	payment := Payment{Status: PaymentStatusSuccess}

	transition := payment.ApplyProviderResult(Payment{Status: PaymentStatusSuccess, TxnID: "wx_txn_1"})

	require.False(t, transition.StatusChanged)
	require.True(t, transition.ShouldPersist)
	require.Equal(t, PaymentStatusSuccess, transition.Payment.Status)
	require.Equal(t, "wx_txn_1", transition.Payment.TxnID)
}

func TestPaymentApplyProviderResultRefundFromSuccess(t *testing.T) {
	payment := Payment{Status: PaymentStatusSuccess, TxnID: "wx_txn_1"}

	transition := payment.ApplyProviderResult(Payment{Status: PaymentStatusRefund})

	require.True(t, transition.StatusChanged)
	require.True(t, transition.ShouldPersist)
	require.Equal(t, PaymentStatusRefund, transition.Payment.Status)
	require.Equal(t, "wx_txn_1", transition.Payment.TxnID)
}
