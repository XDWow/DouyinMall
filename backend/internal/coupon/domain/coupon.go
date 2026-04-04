package domain

import (
	"time"
)

type CouponScope uint8

const (
	CouponScopeUnknown  CouponScope = iota
	CouponScopeAll                  // 鍏ㄥ満鍒革紙涓嶉檺鍒讹級
	CouponScopeMerchant             // 鍟嗗鍒?	CouponScopeCategory             // 鍝佺被鍒?	CouponScopeProduct              // 鍟嗗搧鍒?)

// 浼樻儬鍒哥被鍨?type CouponType uint8

const (
	CouponTypeUnspecified CouponType = iota
	CouponTypeAmount                 // 婊″噺鍒革紙婊?00鍑?0锛?	CouponTypePercent                // 鎶樻墸鍒革紙8鎶橈級
	CouponTypeFixed                  // 绔嬪噺鍒革紙鏃犻棬妲涘噺10鍏冿級
)

// 浼樻儬鍒哥姸鎬?type CouponStatus uint8

func (c CouponStatus) AsUint8() uint8 {
	return uint8(c)
}

const (
	UserCouponStatusUnspecified CouponStatus = iota
	UserCouponStatusUnused                   // 鏈娇鐢?	UserCouponStatusLocked                   // 宸查攣瀹氾紙棰勬墸涓級
	UserCouponStatusUsed                     // 宸蹭娇鐢?	UserCouponStatusExpired                  // 宸茶繃鏈?)

// 浼樻儬鍒告ā鏉匡紙棰嗗煙瀹炰綋锛?// 绠＄悊鍛樺垱寤虹殑浼樻儬瑙勫垯瀹氫箟
type CouponTemplate struct {
	ID   int64
	Name string
	Type CouponType

	Threshold     int64 // 浣跨敤闂ㄦ锛堝垎锛夛紝0 琛ㄧず鏃犻棬妲?	DiscountValue int64 // 閲戦锛堝垎锛夋垨鎶樻墸锛?0=8鎶橈級
	MaxDiscount   int64 // 鎶樻墸鍒告渶澶т紭鎯狅紙鍒嗭級锛? 琛ㄧず涓嶉檺鍒?
	// 浣跨敤鑼冨洿锛堜簰鏂ワ級
	Scope       CouponScope
	MerchantIDs []int64
	CategoryIDs []int64
	ProductIDs  []int64

	// 鏈夋晥鏈燂紙涓ょ妯″紡浜掓枼锛?	ValidDays      int64 // 鐩稿鏈夋晥鏈燂紙澶╋級
	ValidStartTime int64 // 鍥哄畾寮€濮嬫椂闂达紙Unix 绉掞級
	ValidEndTime   int64 // 鍥哄畾缁撴潫鏃堕棿锛圲nix 绉掞級

	// 鍙戞斁闄愬埗
	TotalCount   int32 // 鍙戣鎬婚噺锛?1 琛ㄧず涓嶉檺
	IssuedCount  int32 // 宸插彂鏀炬暟閲忥紙杩愯鎬侊級
	PerUserLimit int32 // 姣忎汉闄愰

	Enabled bool
}

// CanIssue 妫€鏌ユ槸鍚﹀彲浠ュ彂鏀撅紝杩欐槸浼樻儬鍒稿眰闈㈢殑鏄惁鑳藉彂鏀?func (t *CouponTemplate) CanIssue() bool {
	if !t.Enabled {
		return false
	}
	if t.TotalCount > 0 && t.IssuedCount >= t.TotalCount {
		return false
	}
	return true
}

// 璁＄畻鐢ㄦ埛鍒哥殑鏈夋晥鏈?func (t *CouponTemplate) CalculateValidTime() (start, end int64) {
	now := time.Now().Unix()
	if t.ValidDays > 0 {
		// 棰嗗彇鍚嶯澶╂湁鏁?		start = now
		end = now + t.ValidDays*86400
	} else {
		// 鍥哄畾鏃堕棿娈?		start = t.ValidStartTime
		end = t.ValidEndTime
	}
	return
}

type Coupon struct {
	ID             int64
	UserID         int64
	TemplateID     int64
	Template       *CouponTemplate // 鍏宠仈鐨勬ā鏉?	Status         CouponStatus
	OrderID        int64
	ValidStartTime time.Time
	ValidEndTime   time.Time
	CreatedAt      time.Time
	UsedAt         time.Time
}

// DDD:Tell, Don't Ask锛屽憡璇夋垜浣犺鍋氫粈涔堬紝鎴戞潵瑙勭害锛屾垜瑙夊緱鍝簺鐘舵€佸彲浠ヨ浆绉伙紝浣犲憡璇夋垜浣犺鍋氫粈涔?
// Reserve 棰勬墸浼樻儬鍒革紙鐘舵€佽浆绉伙細Unused 鈫?Locked锛?func (c *Coupon) Reserve(orderID int64) error {
	if c.Status != UserCouponStatusUnused {
		return ErrCouponNotAvailable
	}
	c.Status = UserCouponStatusLocked
	c.OrderID = orderID
	return nil
}

// Commit 纭浣跨敤锛堢姸鎬佽浆绉伙細Locked 鈫?Used锛?func (c *Coupon) Commit() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUsed
	return nil
}

// Release 閲婃斁浼樻儬鍒革紙鐘舵€佽浆绉伙細Locked 鈫?Unused锛?func (c *Coupon) Release() error {
	if c.Status != UserCouponStatusLocked {
		return ErrCouponNotLocked
	}
	c.Status = UserCouponStatusUnused
	c.OrderID = 0
	return nil
}

// OrderItem 璁㈠崟鍟嗗搧锛堢敤浜庤绠椾紭鎯犲埜閫傜敤閲戦锛?type OrderItem struct {
	ProductID  int64
	MerchantID int64
	CategoryID int64
	Subtotal   int64 // 璇ュ晢鍝佺殑灏忚閲戦锛堝垎锛?}

// 妫€鏌ヤ紭鎯犲埜鏄惁閫傜敤浜庢煇涓鍗曪紙璁＄畻閫傜敤閲戦 + 妫€鏌ラ棬妲涳級
func (t *CouponTemplate) IsApplicableToOrder(items []OrderItem) (bool, string) {
	// 璁＄畻閫傜敤閲戦锛屾寜鐓cope姹囨€婚噾棰?	applicableAmount := t.CalculateApplicableAmount(items)

	if applicableAmount == 0 {
		return false, "璁㈠崟涓棤閫傜敤鍟嗗搧"
	}
	if applicableAmount < t.Threshold {
		return false, "鏈揪鍒颁娇鐢ㄩ棬妲?
	}

	return true, ""
}

// 璁＄畻浼樻儬鍒稿湪鏈鍗曚腑鐨勯€傜敤閲戦
func (t *CouponTemplate) CalculateApplicableAmount(items []OrderItem) int64 {
	switch t.Scope {
	case CouponScopeAll:
		// 鍏ㄥ満鍒革細鎵€鏈夊晢鍝佸師浠锋€诲拰
		var total int64
		for _, item := range items {
			total += item.Subtotal
		}
		return total

	case CouponScopeMerchant:
		// 鍟嗗鍒革細绛涢€夋寚瀹氬晢瀹剁殑鍟嗗搧
		return sumByIDs(items, t.MerchantIDs, func(item OrderItem) int64 { return item.MerchantID })

	case CouponScopeCategory:
		// 鍝佺被鍒革細绛涢€夋寚瀹氬搧绫荤殑鍟嗗搧
		return sumByIDs(items, t.CategoryIDs, func(item OrderItem) int64 { return item.CategoryID })

	case CouponScopeProduct:
		// 鍟嗗搧鍒革細绛涢€夋寚瀹氬晢鍝?		return sumByIDs(items, t.ProductIDs, func(item OrderItem) int64 { return item.ProductID })

	default:
		return 0
	}
}

// 鑷畾涔?getID 鐨勯€氱敤姹傚拰鏂规硶
func sumByIDs(items []OrderItem, ids []int64, getID func(OrderItem) int64) int64 {
	if len(ids) == 0 {
		return 0
	}
	// 绌洪棿鎹㈡椂闂达紝蹇€熷垽鏂鍗曢」鏄惁鍦╥ds涓?	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	var total int64
	for _, item := range items {
		if _, ok := idSet[getID(item)]; ok {
			total += item.Subtotal
		}
	}
	return total
}

// 浼樻儬鍒告搷浣滆褰曪紝鐢ㄤ簬骞傜瓑鍜屽璁?type CouponOperation struct {
	ID           int64
	OperationID  string // 涓氬姟骞傜瓑閿?	UserCouponID int64
	OrderID      int64
	Type         string // reserve/commit/release/refund
	CreatedAt    int64
}


