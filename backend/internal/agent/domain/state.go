package domain

// 搜索结果中的商品摘要
// 用于对话引用解析（"第一个"→ProductList[0].ProductID）
type ProductSummary struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
}

// 对话涉及的业务对象引用
// 由系统维护，LLM 通过 system prompt 读取名称（不暴露 ID），不由 LLM 负责记忆，因为他记不住
type EntityMemory struct {
	ProductList        []ProductSummary `json:"product_list,omitempty"`         // 最近搜索结果
	CurrentProductID   string           `json:"current_product_id,omitempty"`   // 当前查看商品 ID（backend 使用）
	CurrentProductName string           `json:"current_product_name,omitempty"` // 当前查看商品名（展示给 LLM）
	LastOrderID        string           `json:"last_order_id,omitempty"`        // 最近订单 ID（backend 使用，不暴露给 LLM）
}
