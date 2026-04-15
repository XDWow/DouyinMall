package mcp

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
)

// listUserOrdersFirstPageSize 列表工具仅第一页，条数固定。
const listUserOrdersFirstPageSize int32 = 10

// toFirstPageListCmd 当前用户订单列表第一页（cursor=0，固定条数）。
func toFirstPageListCmd(userID int64) usecase.ListUserOrderCmd {
	return usecase.ListUserOrderCmd{
		UserID: userID,
		Cursor: 0,
		Limit:  listUserOrdersFirstPageSize,
	}
}

// domainOrderToItemView 领域订单 → MCP 摘要一行。
func domainOrderToItemView(o *domain.Order) (QueryOrderItemView, bool) {
	if o == nil {
		return QueryOrderItemView{}, false
	}
	return QueryOrderItemView{
		OrderID:     o.ID,
		Status:      o.Status.String(),
		TotalAmount: o.PayableAmount.Total,
		Currency:    o.PayableAmount.Currency,
		CreatedAt:   o.CreatedAt.Unix(),
	}, true
}

// toQueryOrderItemViews 将本页订单列表转为 MCP 视图。
func toQueryOrderItemViews(orders []*domain.Order) []QueryOrderItemView {
	out := make([]QueryOrderItemView, 0, len(orders))
	for _, o := range orders {
		if v, ok := domainOrderToItemView(o); ok {
			out = append(out, v)
		}
	}
	return out
}
