package domain

import (
	"context"
)

// Repository 鎺ュ彛鎻忚堪鐨勬槸涓氬姟瀵逛笘鐣岀殑鏈熸湜锛岄噸鐐瑰湪涓氬姟锛岃€屼笉鏄熀纭€璁炬柦
// 鎵€浠ュ畠灞炰簬 domain 灞?
// infra 鍙礋璐ｆ彁渚涘疄鐜?
// 杩欐牱涓氬姟灞傛墠鑳藉湪娌℃湁鏁版嵁搴撶殑鎯呭喌涓嬭瀹屾暣寤烘ā鍜屾祴璇?
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, orderID int64) (Order, error)
	FindByIDs(ctx context.Context, orderIDs []int64) ([]*Order, error)
	FindByIDsForUpdate(ctx context.Context, orderIDs []int64) ([]*Order, error)
	UpdateStatus(ctx context.Context, orderID int64, fromStatus, toStatus OrderStatus) error
	ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*Order, error)
	// 鏌ユ壘瓒呰繃30鍒嗛挓鏈敮浠樼殑寰呮敮浠樿鍗曪紙杩囨湡锛?
	FindExpiredOrders(ctx context.Context, limit int) ([]*Order, error)
	// 鎵归噺鏇存柊璁㈠崟鐘舵€侊紝鐜板湪鍙敤浜庢壒閲忓彇娑堣鍗曪細pending -> canceled
	BatchUpdateStatus(ctx context.Context, orderIDs []int64, fromStatus, toStatus OrderStatus) error

	// Keyset鍒嗛〉锛歝ursor涓轰笂涓€椤垫渶鍚庣殑orderID锛岄娆℃煡璇紶0
	// 杩斿洖鍊硷細orders鍒楄〃 + nextCursor锛堢敤浜庝笅涓€椤垫煡璇紝0琛ㄧず娌℃湁鏇村鏁版嵁锛?
	ListByUserID(ctx context.Context, userID int64, cursor int64, limit int) (orders []*Order, nextCursor int64, err error)
}

type OutboxRepository interface {
	Add(ctx context.Context, eventType string, payload any) (int64, error)
	BatchAdd(ctx context.Context, eventType string, payloads []any) ([]int64, error)
	// ListPending 鍒嗛〉鏌ヨ寰呭彂閫佺殑浜嬩欢
	ListPending(ctx context.Context, offset, limit int) ([]OutboxEvent, error)
	MarkSent(ctx context.Context, id int64) error
	BatchMarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64) error
	IncreaseRetry(ctx context.Context, id int64) (int, error)
}


