package domain

type AddToCartInterruptState struct {
	ProductID     string   `json:"product_id,omitempty"`
	ProductName   string   `json:"product_name,omitempty"`
	Spec          string   `json:"spec,omitempty"`
	Quantity      int      `json:"quantity,omitempty"`
	MissingFields []string `json:"missing_fields,omitempty"`
}
