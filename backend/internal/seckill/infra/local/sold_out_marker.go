package local

import (
	"sync"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
)

type SoldOutMarker struct {
	// key: activityID
	soldOut sync.Map
}

func NewSoldOutMarker() domain.SoldOutMarker {
	return &SoldOutMarker{}
}

func (m *SoldOutMarker) IsSoldOut(activityID int64) bool {
	_, ok := m.soldOut.Load(activityID)
	return ok
}

func (m *SoldOutMarker) MarkSoldOut(activityID int64) {
	m.soldOut.Store(activityID, struct{}{})
}

func (m *SoldOutMarker) Clear(activityID int64) {
	m.soldOut.Delete(activityID)
}

// 编译期断言：确保本地标记实现满足领域接口
var _ domain.SoldOutMarker = (*SoldOutMarker)(nil)
