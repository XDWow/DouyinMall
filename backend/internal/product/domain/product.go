package domain

type Product struct {
	ID          int64    `json:"id"`
	SKUID       int64    `json:"sku_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Picture     string   `json:"picture"`
	SlideImgs   []string `json:"slide_image"`
	Price       int64    `json:"price"`
	Currency    string   `json:"currency"`
	Categories  []string `json:"category_id"`
	InStock     bool     `json:"in_stock"`

	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}

type ProductQuery struct {
	ID    int64
	SKUID int64
}
