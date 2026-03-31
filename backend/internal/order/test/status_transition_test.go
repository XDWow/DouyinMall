package test

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/stretchr/testify/require"
)

func TestOrderPayAndRejectInvalidCompletion(t *testing.T) {
	order := &domain.Order{Status: domain.OrderStatusCreated}

	err := order.Pay()
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusPaid, order.Status)

	err = order.Complete()
	require.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
	require.Equal(t, domain.OrderStatusPaid, order.Status)
}

func TestOrderPayIdempotentAfterShipment(t *testing.T) {
	order := &domain.Order{Status: domain.OrderStatusShipped}

	err := order.Pay()
	require.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
	require.Equal(t, domain.OrderStatusShipped, order.Status)
}

func TestOrderShipCompleteCancelAndRefund(t *testing.T) {
	paid := &domain.Order{Status: domain.OrderStatusPaid}
	err := paid.Ship()
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusShipped, paid.Status)

	err = paid.Complete()
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusCompleted, paid.Status)

	created := &domain.Order{Status: domain.OrderStatusCreated}
	err = created.Cancel()
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusCanceled, created.Status)

	refundable := &domain.Order{Status: domain.OrderStatusPaid}
	err = refundable.Refund()
	require.NoError(t, err)
	require.Equal(t, domain.OrderStatusRefunded, refundable.Status)
}

func TestOrderCommandsAreIdempotentOnSameTargetState(t *testing.T) {
	order := &domain.Order{Status: domain.OrderStatusPaid}

	err := order.Pay()
	require.ErrorIs(t, err, domain.ErrOrderStatusUnchanged)
	require.Equal(t, domain.OrderStatusPaid, order.Status)

	order.Status = domain.OrderStatusCanceled
	err = order.Cancel()
	require.ErrorIs(t, err, domain.ErrOrderStatusUnchanged)
	require.Equal(t, domain.OrderStatusCanceled, order.Status)
}
