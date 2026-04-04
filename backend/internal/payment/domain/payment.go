package domain

type Payment struct {
	ID int64

	Amt         Amount
	BizTradeNo  string
	Description string

	Status PaymentStatus
	TxnID  string
}

type Amount struct {
	Currency string
	Total    int64
}

type PaymentStatus uint8

func (s PaymentStatus) AsUint8() uint8 {
	return uint8(s)
}

const (
	PaymentStatusUnknown PaymentStatus = iota
	PaymentStatusInit
	PaymentStatusSuccess
	PaymentStatusFailed
	PaymentStatusRefund
)


