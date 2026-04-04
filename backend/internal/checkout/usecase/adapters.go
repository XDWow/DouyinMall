package usecase

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
)

// IDGenerator 璁㈠崟ID鐢熸垚鍣紙闆姳ID锛?
type IDGenerator interface {
	GenerateOrderID() int64
}

// ==================== Proto 鈫?Domain 杞崲 ====================

// buildOrderLines 灏嗗墠绔紶鍏ョ殑 CheckoutItem 涓?Product 璇︽儏鍚堝苟涓?OrderLine锛?
// 鐩存帴鍦ㄦ瀯寤烘椂鍒嗘祦锛氬彲璐拱鐨勬斁 available锛屽け鏁堢殑鏀?unavailable銆?
// InStock 浣滀负鍙喘涔板垽鏂紝搴撳瓨鏁伴噺鐨勫己鏍￠獙鍦?PlaceOrder.ReserveStock 閲屽仛銆?
func buildOrderLines(items []domain.CheckoutItem, protoProducts []*productv1.Product) (available, unavailable []domain.OrderLine) {
	prodMap := make(map[int64]*productv1.Product, len(protoProducts))
	for _, p := range protoProducts {
		prodMap[p.Id] = p
	}

	for _, item := range items {
		p, ok := prodMap[item.ProductID]
		if !ok {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     item.ProductID,
				Quantity:      item.Quantity,
				Available:     false,
				UnavailReason: "鍟嗗搧涓嶅瓨鍦?,
			})
			continue
		}
		if !p.InStock {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     p.Id,
				Name:          p.Name,
				Picture:       p.Picture,
				Price:         p.Price,
				Currency:      p.Currency,
				Quantity:      item.Quantity,
				Subtotal:      p.Price * item.Quantity,
				Available:     false,
				UnavailReason: "鍟嗗搧宸蹭笅鏋?,
			})
			continue
		}
		available = append(available, domain.OrderLine{
			ProductID: p.Id,
			Name:      p.Name,
			Picture:   p.Picture,
			Price:     p.Price,
			Currency:  p.Currency,
			Quantity:  item.Quantity,
			Subtotal:  p.Price * item.Quantity,
			Available: true,
		})
	}
	return
}

// toCouponOrderItems 浠?OrderLine 鎻愬彇 coupon 鏈嶅姟瑕佹眰鐨?OrderItem
func toCouponOrderItems(lines []domain.OrderLine) []*couponv1.OrderItem {
	result := make([]*couponv1.OrderItem, 0, len(lines))
	for _, l := range lines {
		if !l.Available {
			continue
		}
		result = append(result, &couponv1.OrderItem{
			ProductId: l.ProductID,
			Price:     l.Price,
			Quantity:  int32(l.Quantity),
		})
	}
	return result
}

// toCouponOptions 灏?UserCoupon 鍒楄〃杞负 domain CouponOption锛屽惈姣忚鎽婂垎鐨勪紭鎯犻噾棰濄€?
// lines 鍙寘鍚?Available=true 鐨勮鍗曡锛岀敤浜庢寜姣斾緥鍒嗛厤鎶樻墸銆?
func toCouponOptions(coupons []*couponv1.UserCoupon, lines []domain.OrderLine) []domain.CouponOption {
	// 璁＄畻鍙敤琛岀殑鍟嗗搧鎬讳环
	var productTotal int64
	for _, l := range lines {
		if l.Available {
			productTotal += l.Subtotal
		}
	}

	result := make([]domain.CouponOption, 0, len(coupons))
	for _, c := range coupons {
		opt := domain.CouponOption{
			CouponID: c.Id,
			Usable:   true,
		}
		if c.Template != nil {
			opt.Name = c.Template.Name
			totalDiscount := calculateCouponDiscount(c.Template, productTotal)
			opt.DiscountAmount = totalDiscount
			opt.PerLineDiscounts = allocateDiscountToLines(lines, totalDiscount, productTotal)
		}
		result = append(result, opt)
	}
	return result
}

// calculateCouponDiscount 鏍规嵁鍒告ā鏉跨被鍨嬭绠楁€讳紭鎯犻噾棰?
func calculateCouponDiscount(tmpl *couponv1.CouponTemplate, productTotal int64) int64 {
	switch tmpl.Type {
	case couponv1.CouponType_COUPON_TYPE_AMOUNT: // 婊″噺
		if productTotal >= tmpl.Threshold {
			return tmpl.DiscountValue
		}
		return 0
	case couponv1.CouponType_COUPON_TYPE_FIXED: // 绔嬪噺
		return tmpl.DiscountValue
	case couponv1.CouponType_COUPON_TYPE_PERCENT: // 鎶樻墸锛屽 DiscountValue=80 琛ㄧず8鎶?
		discount := productTotal * (100 - tmpl.DiscountValue) / 100
		if tmpl.MaxDiscount > 0 && discount > tmpl.MaxDiscount {
			discount = tmpl.MaxDiscount
		}
		return discount
	default:
		return 0
	}
}

// allocateDiscountToLines 鎸夎鍗曡灏忚姣斾緥鍒嗛厤鎬绘姌鎵ｏ紝浣欐暟褰掓渶璐电殑琛岋紙閬垮厤鑸嶅叆涓㈠垎锛夈€?
// 杩斿洖 map[ProductID]discountAmount銆?
func allocateDiscountToLines(lines []domain.OrderLine, totalDiscount, productTotal int64) map[int64]int64 {
	if totalDiscount == 0 || productTotal == 0 {
		return nil
	}

	result := make(map[int64]int64, len(lines))
	var allocated int64
	maxSubtotal := int64(-1)
	maxProductID := int64(0)

	for _, l := range lines {
		if !l.Available {
			continue
		}
		// 鎸夋瘮渚嬶細lineDiscount = totalDiscount * lineSubtotal / productTotal
		lineDiscount := totalDiscount * l.Subtotal / productTotal
		result[l.ProductID] = lineDiscount
		allocated += lineDiscount
		// 璁板綍灏忚鏈€澶х殑琛岋紝鐢ㄤ簬鎺ユ敹浣欐暟
		if l.Subtotal > maxSubtotal {
			maxSubtotal = l.Subtotal
			maxProductID = l.ProductID
		}
	}

	// 浣欐暟锛堣垗鍏ヨ宸級褰掓渶璐电殑琛?
	if remainder := totalDiscount - allocated; remainder > 0 && maxProductID != 0 {
		result[maxProductID] += remainder
	}

	return result
}

// toInventoryStockItems 灏?CheckoutItem 杞负 inventory 鏈嶅姟鐨?StockItem
func toInventoryStockItems(items []domain.CheckoutItem) []*inventoryv1.StockItem {
	result := make([]*inventoryv1.StockItem, 0, len(items))
	for _, item := range items {
		result = append(result, &inventoryv1.StockItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
		})
	}
	return result
}

// toOrderAddress 灏?domain Address 杞负 order proto Address
func toOrderAddress(addr domain.Address) *orderv1.Address {
	return &orderv1.Address{
		StreetAddress: addr.Street,
		City:          addr.City,
		State:         addr.Province,
		ZipCode:       addr.ZipCode,
		Phone:         addr.Phone,
	}
}

// toOrderItems 灏?OrderLine 杞负 order proto OrderItem
func toOrderItems(lines []domain.OrderLine, currency string) []*orderv1.OrderItem {
	result := make([]*orderv1.OrderItem, 0, len(lines))
	for _, line := range lines {
		if !line.Available {
			continue
		}
		snapCurrency := line.Currency
		if snapCurrency == "" {
			snapCurrency = currency
		}
		result = append(result, &orderv1.OrderItem{
			ProductId:        line.ProductID,
			Quantity:         line.Quantity,
			SnapshotPrice:    line.Price,
			SnapshotCurrency: snapCurrency,
			ConvertedPrice:   line.Price, // TODO: 姹囩巼杞崲
		})
	}
	return result
}

// ==================== 閫氱敤杈呭姪鍑芥暟 ====================

func extractProductIDs(items []domain.CheckoutItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
	}
	return ids
}

// sumSelectedCouponDiscount 姹囨€荤敤鎴烽€変腑鐨勪紭鎯犲埜鐨勬姌鎵ｉ噾棰?
func sumSelectedCouponDiscount(coupons []domain.CouponOption, selectedIDs []int64) int64 {
	selectedSet := make(map[int64]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		selectedSet[id] = struct{}{}
	}
	var total int64
	for _, c := range coupons {
		if _, ok := selectedSet[c.CouponID]; ok && c.Usable {
			total += c.DiscountAmount
		}
	}
	return total
}

// operationID 鐢熸垚搴撳瓨鎿嶄綔鐨勫箓绛塈D
func operationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

// toInsufficientStockError 灏?inventory 鏈嶅姟杩斿洖鐨勫簱瀛樹笉瓒虫槑缁嗚浆涓?domain 缁撴瀯鍖栭敊璇€?
// lines 鐢ㄤ簬琛ュ厖鍟嗗搧鍚嶇О锛坧roto 鍙湁 product_id锛夈€?
func toInsufficientStockError(items []*inventoryv1.InsufficientItem, lines []domain.OrderLine) *domain.InsufficientStockError {
	nameMap := make(map[int64]string, len(lines))
	for _, l := range lines {
		nameMap[l.ProductID] = l.Name
	}

	result := make([]domain.InsufficientStockItem, len(items))
	for i, item := range items {
		result[i] = domain.InsufficientStockItem{
			ProductID: item.ProductId,
			Name:      nameMap[item.ProductId],
			Requested: int64(item.Requested),
			Available: item.Available,
		}
	}
	return &domain.InsufficientStockError{Items: result}
}

// toCouponUnavailableError 灏?coupon 鏈嶅姟杩斿洖鐨勫け璐ユ槑缁嗚浆涓?domain 缁撴瀯鍖栭敊璇€?
func toCouponUnavailableError(failures []*couponv1.CouponFailure) *domain.CouponUnavailableError {
	items := make([]domain.CouponFailureItem, len(failures))
	for i, f := range failures {
		items[i] = domain.CouponFailureItem{
			CouponID: f.UserCouponId,
			Reason:   f.Reason,
		}
	}
	return &domain.CouponUnavailableError{Failures: items}
}


