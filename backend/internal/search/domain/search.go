package domain

// ==================== 鍟嗗搧鎼滅储 ====================

// ProductSearchResult 鍟嗗搧鎼滅储缁撴灉
type ProductSearchResult struct {
	ID           int64
	Name         string
	Description  string
	Picture      string
	SliderImgs   []string
	Price        int64
	Categories   []string
	InStock      bool
	MerchantID   int64
	MerchantName string
	SalesCount   int64

	Score                float32
	NameHighlight        string
	DescriptionHighlight string
}

// MerchantSearchResult 鍟嗗鎼滅储缁撴灉
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

// SearchSuggestion 鎼滅储寤鸿
type SearchSuggestion struct {
	Keyword string
	Source  string // NAME_MATCH / HISTORY / HOT
	Count   int64
}

// ==================== ES 鏂囨。 ====================

// ProductDocument ES 鍟嗗搧鏂囨。
type ProductDocument struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Picture      string    `json:"picture,omitempty"`
	SliderImgs   []string  `json:"slider_imgs,omitempty"`
	Price        int64     `json:"price"`
	Categories   []string  `json:"categories,omitempty"`
	InStock      bool      `json:"in_stock"`
	MerchantID   int64     `json:"merchant_id"`
	MerchantName string    `json:"merchant_name"`
	SalesCount   int64     `json:"sales_count"`
	NameVector   []float32 `json:"name_vector,omitempty"`
	CreatedTime  int64     `json:"created_at,omitempty"`
	UpdatedTime  int64     `json:"updated_at,omitempty"`
}

// MerchantDocument ES 鍟嗗鏂囨。
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
	CreatedTime  int64   `json:"created_at,omitempty"`
	UpdatedTime  int64   `json:"updated_at,omitempty"`
}

// ==================== 鑱氬悎 ====================

type CategoryAggregation struct {
	Category string
	Count    int64
}

type PriceRangeAggregation struct {
	MinPrice int64
	MaxPrice int64
	Label    string
	Count    int64
}

// ==================== 鍚屾浜嬩欢 ====================

type EventType string

const (
	EventTypeProduct  EventType = "PRODUCT"
	EventTypeMerchant EventType = "MERCHANT"
)

type EventAction string

const (
	EventActionCreate EventAction = "CREATE"
	EventActionUpdate EventAction = "UPDATE"
	EventActionDelete EventAction = "DELETE"
)

type SyncEvent struct {
	Type     EventType         `json:"type"`
	Action   EventAction       `json:"action"`
	ID       int64             `json:"id"`
	Product  *ProductDocument  `json:"product,omitempty"`
	Merchant *MerchantDocument `json:"merchant,omitempty"`
}

// ==================== 鎼滅储璇锋眰/鍝嶅簲 ====================

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


