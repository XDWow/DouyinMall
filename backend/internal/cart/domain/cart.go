package domain

type Cart struct {
	UserID int64      `json:"user_id"`
	Items  []CartItem `json:"items"`
}

type CartItem struct {
	//UserID    int64  `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`

	// domain 涓嶉渶瑕佸惂
	//CreatedAt time.Time `json:"created_at"`
	//UpdatedAt time.Time `json:"updated_at"`

	// 浠?product 鏈嶅姟瀹炴椂鏌ヨ锛岃繖閲屾垜浠笉鐢ㄧ鍟婏紝bff鍘绘煡灏卞ソ浜?
	//Name         string `json:"name"`
	//Picture      string `json:"picture"`
	//Price        int64  `json:"price"`
	//InStock      bool   `json:"in_stock"`
	//MerchantName string `json:"merchant_name"`
}


