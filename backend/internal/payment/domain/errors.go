package domain

import "errors"

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentAlreadyPaid   = errors.New("payment already succeeded")
	ErrPaymentAmountChanged = errors.New("payment amount changed")
)
