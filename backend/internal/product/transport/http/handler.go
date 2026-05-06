package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/product/service"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

type Handler struct {
	svc service.ProductService
}

func NewHandler(svc service.ProductService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	group := engine.Group("/api/products")
	group.GET("", h.ListProducts)
	group.GET("/:productId", h.GetProduct)
}

func (h *Handler) ListProducts(c *gin.Context) {
	page := int64Query(c, "page")
	if page <= 0 {
		page = defaultPage
	}
	pageSize := int64Query(c, "page_size")
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	products, err := h.svc.ListProducts(c.Request.Context(), page, pageSize, c.Query("category"))
	if err != nil {
		writeProductError(c, err)
		return
	}

	items := make([]gin.H, 0, len(products))
	for _, product := range products {
		items = append(items, productToResponse(product))
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"products":  items,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *Handler) GetProduct(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("productId"), 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid product id"})
		return
	}

	product, err := h.svc.GetProduct(c.Request.Context(), productID)
	if err != nil {
		writeProductError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"product": productToResponse(product),
	}})
}

func productToResponse(p domain.Product) gin.H {
	return gin.H{
		"id":            p.ID,
		"name":          p.Name,
		"description":   p.Description,
		"picture":       p.Picture,
		"slider_imgs":   p.SlideImgs,
		"price":         p.Price,
		"categories":    p.Categories,
		"in_stock":      p.InStock,
		"merchant_id":   p.MerchantID,
		"merchant_name": p.MerchantName,
	}
}

func writeProductError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5
	if errors.Is(err, dao.ErrDataNotFound) {
		status = http.StatusNotFound
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
