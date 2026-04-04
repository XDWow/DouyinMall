package domain

type Inventory struct {
	ProductID int64
	Stock     int64
}

// 搴撳瓨鍙樺姩椤?
type StockChange struct {
	ProductID int64
	Quantity  int32 // 鍙樺姩閲忥紙姝ｆ暟=澧炲姞锛岃礋鏁?鍑忓皯锛?
}


