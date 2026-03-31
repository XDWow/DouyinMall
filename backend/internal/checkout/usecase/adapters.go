package usecase

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
)

// IDGenerator 订单ID生成器（雪花ID）
type IDGenerator interface {
	GenerateOrderID() int64
}

// ==================== Proto → Domain 转换 ====================

// buildOrderLines 将前端传入的 CheckoutItem 与 Product 详情合并为 OrderLine，
// 直接在构建时分流：可购买的放 available，失效的放 unavailable。
// InStock 作为可购买判断，库存数量的强校验在 PlaceOrder.ReserveStock 里做。
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
				UnavailReason: "商品不存在",
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
				UnavailReason: "商品已下架",
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

// toCouponOrderItems 从 OrderLine 提取 coupon 服务要求的 OrderItem
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

// toCouponOptions 将 UserCoupon 列表转为 domain CouponOption，含每行摊分的优惠金额。
// lines 只包含 Available=true 的订单行，用于按比例分配折扣。
func toCouponOptions(coupons []*couponv1.UserCoupon, lines []domain.OrderLine) []domain.CouponOption {
	// 计算可用行的商品总价
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

// calculateCouponDiscount 根据券模板类型计算总优惠金额
func calculateCouponDiscount(tmpl *couponv1.CouponTemplate, productTotal int64) int64 {
	switch tmpl.Type {
	case couponv1.CouponType_COUPON_TYPE_AMOUNT: // 满减
		if productTotal >= tmpl.Threshold {
			return tmpl.DiscountValue
		}
		return 0
	case couponv1.CouponType_COUPON_TYPE_FIXED: // 立减
		return tmpl.DiscountValue
	case couponv1.CouponType_COUPON_TYPE_PERCENT: // 折扣，如 DiscountValue=80 表示8折
		discount := productTotal * (100 - tmpl.DiscountValue) / 100
		if tmpl.MaxDiscount > 0 && discount > tmpl.MaxDiscount {
			discount = tmpl.MaxDiscount
		}
		return discount
	default:
		return 0
	}
}

// allocateDiscountToLines 按订单行小计比例分配总折扣，余数归最贵的行（避免舍入丢分）。
// 返回 map[ProductID]discountAmount。
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
		// 按比例：lineDiscount = totalDiscount * lineSubtotal / productTotal
		lineDiscount := totalDiscount * l.Subtotal / productTotal
		result[l.ProductID] = lineDiscount
		allocated += lineDiscount
		// 记录小计最大的行，用于接收余数
		if l.Subtotal > maxSubtotal {
			maxSubtotal = l.Subtotal
			maxProductID = l.ProductID
		}
	}

	// 余数（舍入误差）归最贵的行
	if remainder := totalDiscount - allocated; remainder > 0 && maxProductID != 0 {
		result[maxProductID] += remainder
	}

	return result
}

// toInventoryStockItems 将 CheckoutItem 转为 inventory 服务的 StockItem
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

// toOrderAddress 将 domain Address 转为 order proto Address
func toOrderAddress(addr domain.Address) *orderv1.Address {
	return &orderv1.Address{
		StreetAddress: addr.Street,
		City:          addr.City,
		State:         addr.Province,
		ZipCode:       addr.ZipCode,
		Phone:         addr.Phone,
	}
}

// toOrderItems 将 OrderLine 转为 order proto OrderItem
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
			ConvertedPrice:   line.Price, // TODO: 汇率转换
		})
	}
	return result
}

// ==================== 通用辅助函数 ====================

func extractProductIDs(items []domain.CheckoutItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
	}
	return ids
}

// sumSelectedCouponDiscount 汇总用户选中的优惠券的折扣金额
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

// operationID 生成库存操作的幂等ID
func operationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

// toInsufficientStockError 将 inventory 服务返回的库存不足明细转为 domain 结构化错误。
// lines 用于补充商品名称（proto 只有 product_id）。
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

// toCouponUnavailableError 将 coupon 服务返回的失败明细转为 domain 结构化错误。
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
