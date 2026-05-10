package domain

// SoldOutMarker 只保存本机的“已售罄”快速失败标记，不保存真实库存。
type SoldOutMarker interface {
	IsSoldOut(activityID int64) bool
	MarkSoldOut(activityID int64)
	Clear(activityID int64)
}

// nop 实现用于没有注入本地标记时兜底，保证调用方不用判空。
type nopSoldOutMarker struct{}

func NewNopSoldOutMarker() SoldOutMarker {
	return nopSoldOutMarker{}
}

func (nopSoldOutMarker) IsSoldOut(int64) bool { return false }

func (nopSoldOutMarker) MarkSoldOut(int64) {}

func (nopSoldOutMarker) Clear(int64) {}
