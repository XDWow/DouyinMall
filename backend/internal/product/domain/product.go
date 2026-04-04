package domain

type Product struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Picture     string   `json:"picture"`
	SlideImgs   []string `json:"slide_image"`
	Price       int64    `json:"price"` // 鍗曚綅锛氬垎锛堥伩鍏嶆诞鐐规暟绮惧害闂锛?
	Categories  []string `json:"category_id"`
	InStock     bool     `json:"in_stock"` // 鏄惁鏈夎揣锛堟潵鑷簱瀛樻湇鍔★紝涓嶅瓨鍌ㄥ叿浣撴暟閲忥級

	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}


