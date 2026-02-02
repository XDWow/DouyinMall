package domain

type Inventory struct {
	ProductID int64
	Stock     int64
}

// 库存变动项
type StockChange struct {
	ProductID int64
	Quantity  int32 // 变动量（正数=增加，负数=减少）
}
