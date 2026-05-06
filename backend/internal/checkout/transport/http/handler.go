package http

import (
	"errors"
	"net/http"

	"github.com/XDWow/DouyinMall/backend/internal/checkout/domain"
	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	previewUC *usecase.PreviewOrderUseCase
	placeUC   *usecase.PlaceOrderUseCase
	payUC     *usecase.PayOrderUseCase
}

func NewHandler(
	previewUC *usecase.PreviewOrderUseCase,
	placeUC *usecase.PlaceOrderUseCase,
	payUC *usecase.PayOrderUseCase,
) *Handler {
	return &Handler{
		previewUC: previewUC,
		placeUC:   placeUC,
		payUC:     payUC,
	}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	group := engine.Group("/api/checkout", ginx.RequireGatewayUser())
	group.POST("/preview", h.PreviewOrder)
	group.POST("/place-order", h.PlaceOrder)
	group.POST("/pay-order", h.PayOrder)
}

type checkoutItem struct {
	ProductID int64 `json:"product_id" binding:"required"`
	SKUID     int64 `json:"sku_id" binding:"required"`
	Quantity  int64 `json:"quantity" binding:"required,min=1"`
}

type address struct {
	ReceiverName string `json:"receiver_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Province     string `json:"province" binding:"required"`
	City         string `json:"city" binding:"required"`
	District     string `json:"district" binding:"required"`
	Street       string `json:"street" binding:"required"`
	ZipCode      string `json:"zip_code" binding:"required"`
}

type previewOrderRequest struct {
	Items []checkoutItem `json:"items" binding:"required,min=1"`
}

type placeOrderRequest struct {
	Items          []checkoutItem `json:"items" binding:"required,min=1"`
	Address        address        `json:"address" binding:"required"`
	PaymentMethod  string         `json:"payment_method" binding:"required"`
	Currency       string         `json:"currency" binding:"required"`
	OrderKind      string         `json:"order_kind"`
	Remark         string         `json:"remark"`
	ExpectedAmount int64          `json:"expected_amount" binding:"required"`
	CouponIDs      []int64        `json:"coupon_ids"`
}

type payOrderRequest struct {
	OrderID int64 `json:"order_id" binding:"required"`
}

func (h *Handler) PreviewOrder(c *gin.Context) {
	var req previewOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid preview order request: " + err.Error()})
		return
	}

	output, err := h.previewUC.Execute(c.Request.Context(), usecase.PreviewOrderInput{
		UserID: ginx.GatewayUserID(c),
		Items:  toCheckoutItems(req.Items),
	})
	if err != nil {
		writeCheckoutError(c, err)
		return
	}

	products := make([]gin.H, 0, len(output.Lines))
	for _, line := range output.Lines {
		products = append(products, gin.H{
			"product_id":         line.ProductID,
			"sku_id":             line.SKUID,
			"name":               line.Name,
			"picture":            line.Picture,
			"price":              line.Price,
			"currency":           line.Currency,
			"quantity":           line.Quantity,
			"subtotal":           line.Subtotal,
			"available":          line.Available,
			"unavailable_reason": line.UnavailReason,
		})
	}

	coupons := make([]gin.H, 0, len(output.AvailableCoupons))
	for _, coupon := range output.AvailableCoupons {
		coupons = append(coupons, gin.H{
			"coupon_id":           coupon.CouponID,
			"name":                coupon.Name,
			"description":         coupon.Description,
			"discount_amount":     coupon.DiscountAmount,
			"usable":              coupon.Usable,
			"unusable_reason":     coupon.UnusableReason,
			"per_line_discounts":  coupon.PerLineDiscounts,
		})
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"products":          products,
		"available_coupons": coupons,
	}})
}

func (h *Handler) PlaceOrder(c *gin.Context) {
	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid place order request: " + err.Error()})
		return
	}

	output, err := h.placeUC.Execute(c.Request.Context(), usecase.PlaceOrderInput{
		UserID:         ginx.GatewayUserID(c),
		Items:          toCheckoutItems(req.Items),
		CouponIDs:      req.CouponIDs,
		Address:        toAddress(req.Address),
		PaymentMethod:  req.PaymentMethod,
		Currency:       req.Currency,
		OrderKind:      req.OrderKind,
		Remark:         req.Remark,
		ExpectedAmount: req.ExpectedAmount,
	})
	if err != nil {
		writeCheckoutError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order_id":     output.OrderID,
		"payment_url":  output.PaymentURL,
		"total_amount": output.TotalAmount,
	}})
}

func (h *Handler) PayOrder(c *gin.Context) {
	var req payOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid pay order request: " + err.Error()})
		return
	}

	output, err := h.payUC.Execute(c.Request.Context(), usecase.PayOrderInput{
		UserID:  ginx.GatewayUserID(c),
		OrderID: req.OrderID,
	})
	if err != nil {
		writeCheckoutError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order_id":     output.OrderID,
		"payment_url":  output.PaymentURL,
		"total_amount": output.TotalAmount,
		"expire_at":    output.ExpireAt,
	}})
}

func toCheckoutItems(items []checkoutItem) []domain.CheckoutItem {
	result := make([]domain.CheckoutItem, 0, len(items))
	for _, item := range items {
		result = append(result, domain.CheckoutItem{
			ProductID: item.ProductID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}
	return result
}

func toAddress(addr address) domain.Address {
	return domain.Address{
		ReceiverName: addr.ReceiverName,
		Phone:        addr.Phone,
		Province:     addr.Province,
		City:         addr.City,
		District:     addr.District,
		Street:       addr.Street,
		ZipCode:      addr.ZipCode,
	}
}

func writeCheckoutError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5

	switch {
	case errors.Is(err, domain.ErrOrderForbidden):
		status = http.StatusForbidden
		code = 4
	case errors.Is(err, domain.ErrInvalidInput),
		errors.Is(err, domain.ErrPriceChanged),
		errors.Is(err, domain.ErrInsufficientStock),
		errors.Is(err, domain.ErrOrderNotPayable),
		errors.Is(err, domain.ErrOrderExpired):
		status = http.StatusBadRequest
		code = 4
	case errors.Is(err, domain.ErrOrderCreateFailed),
		errors.Is(err, domain.ErrPaymentCreateFailed):
		status = http.StatusBadGateway
	case isCheckoutClientError(err):
		status = http.StatusBadRequest
		code = 4
	}

	result := ginx.Result{
		Code: code,
		Msg:  err.Error(),
		Data: checkoutErrorData(err),
	}
	c.JSON(status, result)
}

func isCheckoutClientError(err error) bool {
	var unavailable *domain.UnavailableItemsError
	var insufficient *domain.InsufficientStockError
	var couponErr *domain.CouponUnavailableError
	return errors.As(err, &unavailable) || errors.As(err, &insufficient) || errors.As(err, &couponErr)
}

func checkoutErrorData(err error) any {
	var unavailable *domain.UnavailableItemsError
	if errors.As(err, &unavailable) {
		items := make([]gin.H, 0, len(unavailable.Items))
		for _, item := range unavailable.Items {
			items = append(items, gin.H{
				"product_id": item.ProductID,
				"sku_id":     item.SKUID,
				"name":       item.Name,
				"reason":     item.Reason,
			})
		}
		return gin.H{"unavailable_items": items}
	}

	var insufficient *domain.InsufficientStockError
	if errors.As(err, &insufficient) {
		items := make([]gin.H, 0, len(insufficient.Items))
		for _, item := range insufficient.Items {
			items = append(items, gin.H{
				"product_id": item.ProductID,
				"name":       item.Name,
				"requested":  item.Requested,
				"available":  item.Available,
			})
		}
		return gin.H{"insufficient_stock_items": items}
	}

	var couponErr *domain.CouponUnavailableError
	if errors.As(err, &couponErr) {
		items := make([]gin.H, 0, len(couponErr.Failures))
		for _, item := range couponErr.Failures {
			items = append(items, gin.H{
				"coupon_id": item.CouponID,
				"reason":    item.Reason,
			})
		}
		return gin.H{"coupon_failures": items}
	}

	return nil
}
