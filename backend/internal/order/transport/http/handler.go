package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

const (
	defaultListLimit = 10
	maxListLimit     = 50
)

type Handler struct {
	getOrderUC      *usecase.GetOrderUseCase
	listUserOrderUC *usecase.ListUserOrderUseCase
}

func NewHandler(
	getOrderUC *usecase.GetOrderUseCase,
	listUserOrderUC *usecase.ListUserOrderUseCase,
) *Handler {
	return &Handler{
		getOrderUC:      getOrderUC,
		listUserOrderUC: listUserOrderUC,
	}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	group := engine.Group("/api/orders", ginx.RequireGatewayUser())
	group.GET("", h.ListOrders)
	group.GET("/:orderId", h.GetOrder)
}

func (h *Handler) GetOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("orderId"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid order id"})
		return
	}

	order, err := h.getOrderUC.Execute(c.Request.Context(), usecase.GetOrderCmd{OrderID: orderID})
	if err != nil {
		writeOrderError(c, err)
		return
	}

	if order.UserID != ginx.GatewayUserID(c) {
		c.JSON(http.StatusNotFound, ginx.Result{Code: 4, Msg: domain.ErrRecordNotFound.Error()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order": toOrderResponse(order),
	}})
}

func (h *Handler) ListOrders(c *gin.Context) {
	cursor := int64Query(c, "cursor")
	limit := int32(int64Query(c, "limit"))
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	result, err := h.listUserOrderUC.Execute(usecase.ListUserOrderCmd{
		UserID: ginx.GatewayUserID(c),
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}

	orders := make([]gin.H, 0, len(result.Orders))
	for _, order := range result.Orders {
		orders = append(orders, toOrderResponse(order))
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"orders":      orders,
		"next_cursor": result.NextCursor,
	}})
}

func toOrderResponse(order *domain.Order) gin.H {
	items := make([]gin.H, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		items = append(items, gin.H{
			"product_id":        item.ProductID,
			"sku_id":            item.SKUID,
			"quantity":          item.Quantity,
			"snapshot_price":    item.SnapshotPrice,
			"snapshot_currency": item.SnapshotCurrency,
			"converted_price":   item.Price,
		})
	}

	return gin.H{
		"order_id": order.ID,
		"status":   order.Status.String(),
		"amounts": gin.H{
			"currency":        order.PayableAmount.Currency,
			"total_amount":    order.TotalAmount.Total,
			"payable_amount":  order.PayableAmount.Total,
			"discount_amount": order.DiscountAmount.Total,
		},
		"address": gin.H{
			"street":   order.Addr.Street,
			"city":     order.Addr.City,
			"state":    order.Addr.State,
			"country":  order.Addr.Country,
			"zip_code": order.Addr.Zipcode,
			"phone":    order.Addr.Phone,
		},
		"items":       items,
		"remark":      order.Remark,
		"order_kind":  order.OrderKind,
		"activity_id": order.ActivityID,
		"created_at":  order.CreatedAt.Unix(),
		"expire_at":   order.ExpireAt.Unix(),
	}
}

func writeOrderError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5
	if errors.Is(err, domain.ErrRecordNotFound) {
		status = http.StatusNotFound
		code = 4
	} else if err.Error() == "invalid order id" || err.Error() == "invalid list order query" {
		status = http.StatusBadRequest
		code = 4
	}

	c.JSON(status, ginx.Result{
		Code: code,
		Msg:  err.Error(),
	})
}

func int64Query(c *gin.Context, key string) int64 {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}
