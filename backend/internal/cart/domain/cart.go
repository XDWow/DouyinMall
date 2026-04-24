package domain

type Cart struct {
	UserID int64      `json:"user_id"`
	Items  []CartItem `json:"items"`
}

type CartItem struct {
	ProductID int64 `json:"product_id"`
	SKUID     int64 `json:"sku_id"`
	Quantity  int64 `json:"quantity"`
}
