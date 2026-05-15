package domain

import "errors"

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentAlreadyPaid   = errors.New("payment already succeeded")
	ErrPaymentAmountChanged = errors.New("payment amount changed")
	ErrPaymentStatusRace    = errors.New("payment status changed concurrently")
	ErrPaymentTxnIDRequired = errors.New("payment transaction id is required")
	ErrPagePayNotEnabled    = errors.New("page pay is not enabled")
	ErrOrderNotPayable      = errors.New("order is not payable")
	ErrInvalidNotifyData    = errors.New("invalid payment notify data")
)
