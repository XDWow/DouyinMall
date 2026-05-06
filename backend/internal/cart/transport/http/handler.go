package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/cart/domain"
	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc service.CartService
}

func NewHandler(svc service.CartService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	group := engine.Group("/api/cart", ginx.RequireGatewayUser())
	group.GET("", h.GetCart)
	group.DELETE("", h.EmptyCart)
	group.POST("/items", h.AddItems)
	group.DELETE("/items", h.DeleteItems)
	group.DELETE("/items/:skuId", h.DeleteItem)
	group.PUT("/items/:skuId", h.ChangeQty)
	group.POST("/items/:skuId/increment", h.IncrementQty)
	group.POST("/items/:skuId/decrement", h.DecrementQty)
}

type cartItem struct {
	ProductID int64 `json:"product_id" binding:"required"`
	SKUID     int64 `json:"sku_id" binding:"required"`
	Quantity  int64 `json:"quantity" binding:"required,min=1"`
}

type addItemsRequest struct {
	Items []cartItem `json:"items" binding:"required,min=1"`
}

type changeQtyRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity" binding:"required,min=1"`
}

func (h *Handler) GetCart(c *gin.Context) {
	cart, err := h.svc.GetCart(c.Request.Context(), ginx.GatewayUserID(c))
	if err != nil {
		writeCartError(c, err)
		return
	}

	items := make([]gin.H, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, gin.H{
			"product_id": item.ProductID,
			"sku_id":     item.SKUID,
			"quantity":   item.Quantity,
		})
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"cart": gin.H{
			"user_id": cart.UserID,
			"items":   items,
		},
	}})
}

func (h *Handler) AddItems(c *gin.Context) {
	var req addItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid add items request: " + err.Error()})
		return
	}

	items := make([]domain.CartItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, domain.CartItem{
			ProductID: item.ProductID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}

	if err := h.svc.AddItems(c.Request.Context(), ginx.GatewayUserID(c), items); err != nil {
		writeCartError(c, err)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
}

func (h *Handler) DeleteItems(c *gin.Context) {
	skuIDs := parseSKUIds(c.QueryArray("sku_ids"))
	if len(skuIDs) == 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "sku_ids is required"})
		return
	}

	if err := h.svc.DeleteItems(c.Request.Context(), ginx.GatewayUserID(c), skuIDs); err != nil {
		writeCartError(c, err)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
}

func (h *Handler) DeleteItem(c *gin.Context) {
	skuID, err := strconv.ParseInt(c.Param("skuId"), 10, 64)
	if err != nil || skuID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid sku id"})
		return
	}

	if err := h.svc.DeleteItems(c.Request.Context(), ginx.GatewayUserID(c), []int64{skuID}); err != nil {
		writeCartError(c, err)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
}

func (h *Handler) EmptyCart(c *gin.Context) {
	if err := h.svc.EmptyCart(c.Request.Context(), ginx.GatewayUserID(c)); err != nil {
		writeCartError(c, err)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
}

func (h *Handler) ChangeQty(c *gin.Context) {
	skuID, err := strconv.ParseInt(c.Param("skuId"), 10, 64)
	if err != nil || skuID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid sku id"})
		return
	}

	var req changeQtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid change quantity request: " + err.Error()})
		return
	}

	if err := h.svc.ChangeQty(c.Request.Context(), ginx.GatewayUserID(c), domain.CartItem{
		ProductID: req.ProductID,
		SKUID:     skuID,
		Quantity:  req.Quantity,
	}); err != nil {
		writeCartError(c, err)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
}

func (h *Handler) IncrementQty(c *gin.Context) {
	h.adjustQty(c, true)
}

func (h *Handler) DecrementQty(c *gin.Context) {
	h.adjustQty(c, false)
}

func (h *Handler) adjustQty(c *gin.Context, increment bool) {
	skuID, err := strconv.ParseInt(c.Param("skuId"), 10, 64)
	if err != nil || skuID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid sku id"})
		return
	}

	var newQty int64
	if increment {
		newQty, err = h.svc.IncrementQty(c.Request.Context(), ginx.GatewayUserID(c), skuID)
	} else {
		newQty, err = h.svc.DecrementQty(c.Request.Context(), ginx.GatewayUserID(c), skuID)
	}
	if err != nil {
		writeCartError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"sku_id":       skuID,
		"new_quantity": newQty,
	}})
}

func parseSKUIds(values []string) []int64 {
	var result []int64
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			value, err := strconv.ParseInt(part, 10, 64)
			if err != nil || value <= 0 {
				continue
			}
			result = append(result, value)
		}
	}
	return result
}

func writeCartError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, ginx.Result{
		Code: 5,
		Msg:  err.Error(),
	})
}
