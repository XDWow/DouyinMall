package domain

// 商品搜索结果
type ProductSearchResult struct {
	ID           int64
	Name         string
	Description  string
	Picture      string
	SliderImgs   []string
	Price        int64
	Categories   []string
	InStock      bool // 是否有货（来自库存服务）
	MerchantID   int64
	MerchantName string

	Score                float32
	NameHighlight        string
	DescriptionHighlight string
}

// 商家搜索结果
type MerchantSearchResult struct {
	ID           int64
	Name         string
	Description  string
	Logo         string
	Region       string
	Rating       float32
	SalesCount   int64
	ProductCount int64
	Verified     bool

	Score         float32
	NameHighlight string
}

// 搜索的建议
type SearchSuggestion struct {
	Keyword string
	Source  string // 来源：匹配，历史，热门
	Count   int64
}

// 商品文档
type ProductDocument struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Picture      string   `json:"picture,omitempty"`
	SliderImgs   []string `json:"slider_imgs,omitempty"`
	Price        int64    `json:"price"`
	Categories   []string `json:"categories,omitempty"`
	InStock      bool     `json:"in_stock"`
	MerchantID   int64    `json:"merchant_id"`
	MerchantName string   `json:"merchant_name"`

	CreatedTime int64 `json:"created_at,omitempty"`
	UpdatedTime int64 `json:"updated_at,omitempty"`
}

// 商家
type MerchantDocument struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Logo         string  `json:"logo,omitempty"`
	Region       string  `json:"region,omitempty"`
	Rating       float32 `json:"rating"`
	SalesCount   int64   `json:"sales_count"`
	ProductCount int64   `json:"product_count"`
	Verified     bool    `json:"verified"`

	CreatedTime int64 `json:"created_at,omitempty"`
	UpdatedTime int64 `json:"updated_at,omitempty"`
}

// 分类聚合统计
type CategoryAggregation struct {
	Category string
	Count    int64
}

// 价格区间统计
type PriceRangeAggregation struct {
	// 点击筛选后，传递给后端，不依赖label解析（麻烦且易出错）
	MinPrice int64
	MaxPrice int64
	// 用于展示
	Label string

	Count int64
}

// EventType 事件类型
type EventType string

const (
	EventTypeProduct  EventType = "PRODUCT"  // 商品事件
	EventTypeMerchant EventType = "MERCHANT" // 商家事件
)

// EventAction 事件操作类型
type EventAction string

const (
	EventActionCreate EventAction = "CREATE" // 创建
	EventActionUpdate EventAction = "UPDATE" // 更新
	EventActionDelete EventAction = "DELETE" // 删除
)

type SyncEvent struct {
	Type   EventType   `json:"type"`   // 事件类型：PRODUCT 或 MERCHANT
	Action EventAction `json:"action"` // 操作类型：CREATE、UPDATE、DELETE
	ID     int64       `json:"id"`     // 实体 ID（DELETE 时必需）

	// 根据 Type 决定使用哪个字段
	Product  *ProductDocument  `json:"product,omitempty"`
	Merchant *MerchantDocument `json:"merchant,omitempty"`
}

// 搜索请求和响应
type SearchProductsReq struct {
	Keyword         string
	Page            int64
	PageSize        int64
	Categories      []string
	MinPrice        int64
	MaxPrice        int64
	MerchantID      int64
	InStockOnly     bool
	SortBy          string
	EnableHighlight bool
}

type SearchProductsResp struct {
	Products []ProductSearchResult
	Total    int64
	Page     int64
	PageSize int64
}

type SearchMerchantsReq struct {
	Keyword   string
	Page      int64
	PageSize  int64
	Region    string
	MinRating float32
	Verified  *bool
	SortBy    string
}

type SearchMerchantsResp struct {
	Merchants []MerchantSearchResult
	Total     int64
	Page      int64
	PageSize  int64
}

type SearchAggregationsResp struct {
	Categories  []CategoryAggregation
	PriceRanges []PriceRangeAggregation
}
