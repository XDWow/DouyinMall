package usecase

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
)

type IDGenerator interface {
	GenerateOrderID() int64
}

type productLineKey struct {
	productID int64
	skuID     int64
}

func buildOrderLines(items []domain.CheckoutItem, protoProducts []*productv1.Product) (available, unavailable []domain.OrderLine) {
	prodMap := make(map[productLineKey]*productv1.Product, len(protoProducts))
	for _, p := range protoProducts {
		if p == nil {
			continue
		}
		prodMap[productLineKey{productID: p.GetId(), skuID: p.GetSkuId()}] = p
	}

	for _, item := range items {
		p, ok := prodMap[productLineKey{productID: item.ProductID, skuID: item.SKUID}]
		if !ok {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     item.ProductID,
				SKUID:         item.SKUID,
				Quantity:      item.Quantity,
				Available:     false,
				UnavailReason: "product not found",
			})
			continue
		}
		if !p.GetInStock() {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     p.GetId(),
				SKUID:         item.SKUID,
				Name:          p.GetName(),
				Picture:       p.GetPicture(),
				Price:         p.GetPrice(),
				Currency:      p.GetCurrency(),
				Quantity:      item.Quantity,
				Subtotal:      p.GetPrice() * item.Quantity,
				Available:     false,
				UnavailReason: "product out of stock",
			})
			continue
		}
		available = append(available, domain.OrderLine{
			ProductID: p.GetId(),
			SKUID:     item.SKUID,
			Name:      p.GetName(),
			Picture:   p.GetPicture(),
			Price:     p.GetPrice(),
			Currency:  p.GetCurrency(),
			Quantity:  item.Quantity,
			Subtotal:  p.GetPrice() * item.Quantity,
			Available: true,
		})
	}
	return
}

func buildOrderLinesFromQuotes(items []domain.CheckoutItem, protoQuotes []*productv1.ProductQuote) (available, unavailable []domain.OrderLine) {
	quoteMap := make(map[productLineKey]*productv1.ProductQuote, len(protoQuotes))
	for _, q := range protoQuotes {
		if q == nil {
			continue
		}
		quoteMap[productLineKey{productID: q.GetProductId(), skuID: q.GetSkuId()}] = q
	}

	for _, item := range items {
		q, ok := quoteMap[productLineKey{productID: item.ProductID, skuID: item.SKUID}]
		if !ok {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     item.ProductID,
				SKUID:         item.SKUID,
				Quantity:      item.Quantity,
				Available:     false,
				UnavailReason: "product not found",
			})
			continue
		}
		if !q.GetInStock() {
			unavailable = append(unavailable, domain.OrderLine{
				ProductID:     q.GetProductId(),
				SKUID:         q.GetSkuId(),
				Price:         q.GetPrice(),
				Currency:      q.GetCurrency(),
				Quantity:      item.Quantity,
				Subtotal:      q.GetPrice() * item.Quantity,
				Available:     false,
				UnavailReason: "product out of stock",
			})
			continue
		}
		available = append(available, domain.OrderLine{
			ProductID: q.GetProductId(),
			SKUID:     q.GetSkuId(),
			Price:     q.GetPrice(),
			Currency:  q.GetCurrency(),
			Quantity:  item.Quantity,
			Subtotal:  q.GetPrice() * item.Quantity,
			Available: true,
		})
	}
	return
}

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

func toCouponOptions(coupons []*couponv1.UserCoupon, lines []domain.OrderLine) []domain.CouponOption {
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

func calculateCouponDiscount(tmpl *couponv1.CouponTemplate, productTotal int64) int64 {
	switch tmpl.Type {
	case couponv1.CouponType_COUPON_TYPE_AMOUNT:
		if productTotal >= tmpl.Threshold {
			return tmpl.DiscountValue
		}
		return 0
	case couponv1.CouponType_COUPON_TYPE_FIXED:
		return tmpl.DiscountValue
	case couponv1.CouponType_COUPON_TYPE_PERCENT:
		discount := productTotal * (100 - tmpl.DiscountValue) / 100
		if tmpl.MaxDiscount > 0 && discount > tmpl.MaxDiscount {
			discount = tmpl.MaxDiscount
		}
		return discount
	default:
		return 0
	}
}

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
		lineDiscount := totalDiscount * l.Subtotal / productTotal
		result[l.ProductID] = lineDiscount
		allocated += lineDiscount
		if l.Subtotal > maxSubtotal {
			maxSubtotal = l.Subtotal
			maxProductID = l.ProductID
		}
	}

	if remainder := totalDiscount - allocated; remainder > 0 && maxProductID != 0 {
		result[maxProductID] += remainder
	}

	return result
}

func toInventoryStockItems(items []domain.CheckoutItem) []*inventoryv1.StockItem {
	result := make([]*inventoryv1.StockItem, 0, len(items))
	indexByProductID := make(map[int64]int, len(items))
	for _, item := range items {
		if idx, ok := indexByProductID[item.ProductID]; ok {
			result[idx].Quantity += int32(item.Quantity)
			continue
		}
		result = append(result, &inventoryv1.StockItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
		})
		indexByProductID[item.ProductID] = len(result) - 1
	}
	return result
}

func toOrderAddress(addr domain.Address) *orderv1.Address {
	return &orderv1.Address{
		StreetAddress: addr.Street,
		City:          addr.City,
		State:         addr.Province,
		ZipCode:       addr.ZipCode,
		Phone:         addr.Phone,
	}
}

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
			SkuId:            line.SKUID,
			Quantity:         line.Quantity,
			SnapshotPrice:    line.Price,
			SnapshotCurrency: snapCurrency,
			ConvertedPrice:   line.Price,
		})
	}
	return result
}

func extractProductQueries(items []domain.CheckoutItem) []*productv1.ProductQuery {
	queries := make([]*productv1.ProductQuery, 0, len(items))
	for _, item := range items {
		queries = append(queries, &productv1.ProductQuery{
			ProductId: item.ProductID,
			SkuId:     item.SKUID,
		})
	}
	return queries
}

func validateCheckoutItems(items []domain.CheckoutItem) error {
	for _, item := range items {
		if item.ProductID <= 0 {
			return fmt.Errorf("product_id is required")
		}
		if item.SKUID <= 0 {
			return fmt.Errorf("sku_id is required")
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("quantity must be positive")
		}
	}
	return nil
}

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

func operationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

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
