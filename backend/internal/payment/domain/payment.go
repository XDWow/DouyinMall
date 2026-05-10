package domain

type Payment struct {
	ID int64

	Amt         Amount
	BizTradeNo  string
	Description string

	Status PaymentStatus
	TxnID  string
}

type PaymentTransition struct {
	Payment       Payment
	StatusChanged bool
	ShouldPersist bool
}

func (p Payment) ApplyProviderResult(incoming Payment) PaymentTransition {
	next := p
	if incoming.TxnID != "" && next.TxnID == "" {
		next.TxnID = incoming.TxnID
	}

	if incoming.Status == PaymentStatusUnknown || incoming.Status == PaymentStatusInit {
		return PaymentTransition{
			Payment:       next,
			ShouldPersist: next.TxnID != p.TxnID,
		}
	}

	nextStatus, statusChanged := nextPaymentStatus(p.Status, incoming.Status)
	next.Status = nextStatus
	return PaymentTransition{
		Payment:       next,
		StatusChanged: statusChanged,
		ShouldPersist: statusChanged || next.TxnID != p.TxnID,
	}
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

func nextPaymentStatus(current, incoming PaymentStatus) (PaymentStatus, bool) {
	switch current {
	case PaymentStatusSuccess:
		if incoming == PaymentStatusRefund {
			return PaymentStatusRefund, true
		}
		return current, false
	case PaymentStatusRefund:
		return current, false
	case PaymentStatusFailed:
		if incoming == PaymentStatusSuccess {
			return PaymentStatusSuccess, true
		}
		return current, false
	default:
		return incoming, true
	}
}
