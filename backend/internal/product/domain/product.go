package domain

type Product struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Picture     string   `json:"picture"`
	SlideImgs   []string `json:"slide_image"`
	Price       int64    `json:"price"` // 单位：分（避免浮点数精度问题）
	Categories  []string `json:"category_id"`
	Stock       int64    `json:"stock"`

	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}
