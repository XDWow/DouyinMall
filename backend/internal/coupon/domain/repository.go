package domain

import "context"

type CouponTemplateRepository interface {
	GetByID(ctx context.Context, id int64) (CouponTemplate, error)
	// 澧炲姞宸插彂鏀炬暟閲忥紙鍘熷瓙鎿嶄綔锛?	IncrIssuedCount(ctx context.Context, id int64) error
}

type CouponRepository interface {
	// 鍙戞斁浼樻儬鍒?	Issue(ctx context.Context, coupon *Coupon) (int64, error)
	// 鍒嗛〉鏌ヨ鐢ㄦ埛浼樻儬鍒?	ListByUserID(ctx context.Context, userID int64, status CouponStatus, page, pageSize int) ([]*Coupon, int32, error)
	// 鏌ヨ鐢ㄦ埛鍙敤浼樻儬鍒?	ListAvailableByUserID(ctx context.Context, userID int64) ([]*Coupon, error)
	// 鏍规嵁鎸囧畾ID鏌ヨ鍙敤鍒革紙楠岃瘉骞惰繃婊わ細鐘舵€併€佹湁鏁堟湡銆佸綊灞烇級
	GetAvailableByIDs(ctx context.Context, userID int64, couponIDs []int64) ([]*Coupon, error)
	// 缁熻鐢ㄦ埛宸查鍙栨煇妯℃澘鐨勬暟閲?	CountByUserAndTemplate(ctx context.Context, userID, templateID int64) (int32, error)

	// 鎵归噺棰勫崰浼樻儬鍒革紙Unused 鈫?Locked锛?	BatchReserve(ctx context.Context, couponIDs []int64, orderID int64) error
	// 鏍规嵁璁㈠崟ID鏇存柊鐘舵€侊紝浣跨敤鍦烘櫙锛?	//   - Commit:  fromStatus=Locked, toStatus=Used
	//   - Release: fromStatus=Locked, toStatus=Unused
	//   - Refund:  fromStatus=Used, toStatus=Unused
	UpdateStatusByOrderID(ctx context.Context, orderID int64, fromStatus, toStatus CouponStatus) error

	// 鎵归噺鏍囪杩囨湡浼樻儬鍒革紙杩斿洖褰卞搷琛屾暟锛岀敤浜庢棩蹇楋級
	MarkExpiredCoupons(ctx context.Context) (int64, error)
}

// 鍙戝埜骞傜瓑
type CouponOperationRepository interface {
	// 鍒涘缓鎿嶄綔璁板綍锛堝敮涓€绱㈠紩淇濊瘉骞傜瓑锛?	Create(ctx context.Context, op *CouponOperation) error
	// 鏍规嵁骞傜瓑閿煡璇㈡搷浣滆褰曪紝杩斿洖宸插彂鏀剧殑鍒窱D锛堢敤浜庡箓绛夐噸璇曪級
	GetByOperationID(ctx context.Context, operationID string) (*CouponOperation, error)
}


