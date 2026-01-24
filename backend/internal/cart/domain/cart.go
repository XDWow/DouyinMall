package domain

type Cart struct {
	UserID int64      `json:"user_id"`
	Items  []CartItem `json:"items"`
}

type CartItem struct {
	//UserID    int64  `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`

	// domain 不需要吧
	//CreatedAt time.Time `json:"created_at"`
	//UpdatedAt time.Time `json:"updated_at"`

	// 从 product 服务实时查询，这里我们不用管啊，bff去查就好了
	//Name         string `json:"name"`
	//Picture      string `json:"picture"`
	//Price        int64  `json:"price"`
	//InStock      bool   `json:"in_stock"`
	//MerchantName string `json:"merchant_name"`
}
