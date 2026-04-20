package domain

type AftersalesApplyInterruptState struct {
	OrderID       string   `json:"order_id,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	RequestType   string   `json:"request_type,omitempty"`
	MissingFields []string `json:"missing_fields,omitempty"`
}
