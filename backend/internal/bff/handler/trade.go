package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	checkoutv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/checkout/v1/checkoutservice"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	productv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1/seckillservice"
	"github.com/gin-gonic/gin"
)

type TradeHandler struct {
	checkoutClient  checkoutservice.Client
	seckillClient   seckillservice.Client
	orderClient     orderservice.Client
	productClient   productservice.Client
	inventoryClient inventoryservice.Client
}

func NewTradeHandler(
	checkoutClient checkoutservice.Client,
	seckillClient seckillservice.Client,
	orderClient orderservice.Client,
	productClient productservice.Client,
	inventoryClient inventoryservice.Client,
) *TradeHandler {
	return &TradeHandler{
		checkoutClient:  checkoutClient,
		seckillClient:   seckillClient,
		orderClient:     orderClient,
		productClient:   productClient,
		inventoryClient: inventoryClient,
	}
}

func (h *TradeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	checkoutGroup := rg.Group("/checkout")
	checkoutGroup.POST("/place-order", h.PlaceOrder)
	checkoutGroup.POST("/pay-order", h.PayOrder)

	orderGroup := rg.Group("/orders")
	orderGroup.GET("/:orderId", h.GetOrder)

	seckillGroup := rg.Group("/seckill")
	seckillGroup.POST("/activities", h.CreateActivity)
	seckillGroup.GET("/activities/:activityId", h.GetActivity)
	seckillGroup.POST("/submit", h.SubmitSeckill)
	seckillGroup.GET("/result", h.GetSeckillResult)

	testingGroup := rg.Group("/testing")
	testingGroup.POST("/products", h.CreateProductWithStock)
}

type placeOrderRequest struct {
	UserID         int64        `json:"user_id"`
	Items          []tradeItem  `json:"items" binding:"required,min=1"`
	Address        tradeAddress `json:"address" binding:"required"`
	PaymentMethod  string       `json:"payment_method" binding:"required"`
	Currency       string       `json:"currency" binding:"required"`
	OrderKind      string       `json:"order_kind"`
	Remark         string       `json:"remark"`
	ExpectedAmount int64        `json:"expected_amount" binding:"required"`
	CouponIDs      []int64      `json:"coupon_ids"`
}

type payOrderRequest struct {
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id" binding:"required"`
}

type tradeItem struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int64 `json:"quantity" binding:"required,min=1"`
}

type tradeAddress struct {
	ReceiverName string `json:"receiver_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Province     string `json:"province" binding:"required"`
	City         string `json:"city" binding:"required"`
	District     string `json:"district" binding:"required"`
	Street       string `json:"street" binding:"required"`
	ZipCode      string `json:"zip_code" binding:"required"`
}

type createActivityRequest struct {
	ActivityName string `json:"activity_name" binding:"required"`
	ProductID    int64  `json:"product_id" binding:"required"`
	SKUID        int64  `json:"sku_id" binding:"required"`
	SeckillPrice int64  `json:"seckill_price" binding:"required"`
	TotalStock   int32  `json:"total_stock" binding:"required"`
	StartTime    int64  `json:"start_time" binding:"required"`
	EndTime      int64  `json:"end_time" binding:"required"`
	Status       string `json:"status" binding:"required"`
	LimitPerUser int32  `json:"limit_per_user" binding:"required"`
}

type submitSeckillRequest struct {
	UserID     int64 `json:"user_id"`
	ActivityID int64 `json:"activity_id" binding:"required"`
}

type createTestingProductRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Picture     string `json:"picture"`
	Price       int64  `json:"price" binding:"required"`
	Currency    string `json:"currency" binding:"required"`
	Stock       int32  `json:"stock" binding:"required"`
}

func (h *TradeHandler) PlaceOrder(c *gin.Context) {
	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid place order request: " + err.Error()})
		return
	}

	resp, err := h.checkoutClient.PlaceOrder(c.Request.Context(), &checkoutv1.PlaceOrderReq{
		UserId:         requestUserID(c, req.UserID),
		Items:          toCheckoutItems(req.Items),
		CouponIds:      req.CouponIDs,
		Address:        toCheckoutAddress(req.Address),
		PaymentMethod:  req.PaymentMethod,
		Currency:       req.Currency,
		OrderKind:      req.OrderKind,
		Remark:         req.Remark,
		ExpectedAmount: req.ExpectedAmount,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "place order failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order_id":     resp.GetOrderId(),
		"payment_url":  resp.GetPaymentUrl(),
		"total_amount": resp.GetTotalAmount(),
		"expire_at":    resp.GetExpireAt(),
	}})
}

func (h *TradeHandler) PayOrder(c *gin.Context) {
	var req payOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid pay order request: " + err.Error()})
		return
	}

	resp, err := h.checkoutClient.PayOrder(c.Request.Context(), &checkoutv1.PayOrderReq{
		UserId:  requestUserID(c, req.UserID),
		OrderId: req.OrderID,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "pay order failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order_id":     resp.GetOrderId(),
		"payment_url":  resp.GetPaymentUrl(),
		"total_amount": resp.GetTotalAmount(),
		"expire_at":    resp.GetExpireAt(),
	}})
}

func (h *TradeHandler) GetOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("orderId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid order id"})
		return
	}

	resp, err := h.orderClient.GetOrder(c.Request.Context(), &orderv1.GetOrderReq{OrderId: orderID})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "get order failed: " + err.Error()})
		return
	}
	order := resp.GetOrder()
	if order == nil {
		c.JSON(http.StatusNotFound, ginx.Result{Code: 4, Msg: "order not found"})
		return
	}

	items := make([]gin.H, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, gin.H{
			"product_id":        item.GetProductId(),
			"sku_id":            item.GetSkuId(),
			"quantity":          item.GetQuantity(),
			"snapshot_price":    item.GetSnapshotPrice(),
			"snapshot_currency": item.GetSnapshotCurrency(),
			"converted_price":   item.GetConvertedPrice(),
		})
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"order_id":     order.GetOrderId(),
		"user_id":      order.GetUserId(),
		"order_status": order.GetOrderStatus().String(),
		"order_kind":   order.GetOrderKind(),
		"activity_id":  order.GetActivityId(),
		"total_amount": order.GetTotalAmount(),
		"currency":     order.GetCurrency(),
		"address": gin.H{
			"street_address": order.GetAddress().GetStreetAddress(),
			"city":           order.GetAddress().GetCity(),
			"state":          order.GetAddress().GetState(),
			"country":        order.GetAddress().GetCountry(),
			"zip_code":       order.GetAddress().GetZipCode(),
			"phone":          order.GetAddress().GetPhone(),
		},
		"remark":     order.GetRemark(),
		"created_at": order.GetCreatedAt(),
		"expire_at":  order.GetExpireAt(),
		"items":      items,
	}})
}

func (h *TradeHandler) CreateActivity(c *gin.Context) {
	var req createActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid create activity request: " + err.Error()})
		return
	}

	resp, err := h.seckillClient.CreateActivity(c.Request.Context(), &seckillv1.CreateActivityReq{
		ActivityName: req.ActivityName,
		ProductId:    req.ProductID,
		SkuId:        req.SKUID,
		SeckillPrice: req.SeckillPrice,
		TotalStock:   req.TotalStock,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Status:       req.Status,
		LimitPerUser: req.LimitPerUser,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "create seckill activity failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"activity_id": resp.GetActivityId()}})
}

func (h *TradeHandler) GetActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("activityId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid activity id"})
		return
	}

	resp, err := h.seckillClient.GetActivity(c.Request.Context(), &seckillv1.GetActivityReq{ActivityId: activityID})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "get activity failed: " + err.Error()})
		return
	}
	activity := resp.GetActivity()
	if activity == nil {
		c.JSON(http.StatusNotFound, ginx.Result{Code: 4, Msg: "activity not found"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"id":              activity.GetId(),
		"activity_name":   activity.GetActivityName(),
		"product_id":      activity.GetProductId(),
		"sku_id":          activity.GetSkuId(),
		"seckill_price":   activity.GetSeckillPrice(),
		"total_stock":     activity.GetTotalStock(),
		"available_stock": activity.GetAvailableStock(),
		"start_time":      activity.GetStartTime(),
		"end_time":        activity.GetEndTime(),
		"status":          activity.GetStatus(),
		"limit_per_user":  activity.GetLimitPerUser(),
	}})
}

func (h *TradeHandler) SubmitSeckill(c *gin.Context) {
	var req submitSeckillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid submit seckill request: " + err.Error()})
		return
	}

	resp, err := h.seckillClient.SubmitSeckill(c.Request.Context(), &seckillv1.SubmitSeckillReq{
		ActivityId: req.ActivityID,
		UserId:     requestUserID(c, req.UserID),
	})
	if err != nil && resp == nil {
		if result := deriveSeckillFailResult(err); result != nil {
			c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: result})
			return
		}
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "submit seckill failed: " + err.Error()})
		return
	}

	data := gin.H{
		"request_no": resp.GetRequestNo(),
		"status":     resp.GetStatus(),
		"message":    resp.GetMessage(),
	}
	if err != nil {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: data, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: data})
}

func deriveSeckillFailResult(err error) gin.H {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "out of stock"):
		return gin.H{"request_no": "", "status": "FAIL", "message": "OUT_OF_STOCK"}
	case strings.Contains(msg, "duplicate"):
		return gin.H{"request_no": "", "status": "FAIL", "message": "DUPLICATE"}
	case strings.Contains(msg, "not started"), strings.Contains(msg, "offline"), strings.Contains(msg, "ended"):
		return gin.H{"request_no": "", "status": "FAIL", "message": "ACTIVITY_NOT_OPEN"}
	default:
		return nil
	}
}

func (h *TradeHandler) GetSeckillResult(c *gin.Context) {
	requestNo := c.Query("request_no")
	if requestNo == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "request_no is required"})
		return
	}

	resp, err := h.seckillClient.GetSeckillResult(c.Request.Context(), &seckillv1.GetSeckillResultReq{
		RequestNo: requestNo,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "get seckill result failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"request_no":  resp.GetRequestNo(),
		"status":      resp.GetStatus(),
		"order_id":    resp.GetOrderId(),
		"fail_reason": resp.GetFailReason(),
	}})
}

func (h *TradeHandler) CreateProductWithStock(c *gin.Context) {
	var req createTestingProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid testing product request: " + err.Error()})
		return
	}

	createResp, err := h.productClient.CreateProduct(c.Request.Context(), &productv1.CreateProductReq{
		Product: &productv1.Product{
			Name:         req.Name,
			Description:  req.Description,
			Picture:      req.Picture,
			Price:        req.Price,
			Currency:     req.Currency,
			Categories:   []string{"benchmark"},
			InStock:      true,
			MerchantID:   10001,
			MerchantName: "bff-benchmark",
		},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "create product failed: " + err.Error()})
		return
	}

	adjustResp, err := h.inventoryClient.AdjustStock(c.Request.Context(), &inventoryv1.AdjustStockReq{
		Reason: "bff_testing_seed_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Items: []*inventoryv1.StockItem{{
			ProductId: createResp.GetId(),
			Quantity:  req.Stock,
		}},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, ginx.Result{Code: 5, Msg: "seed stock failed: " + err.Error()})
		return
	}
	if adjustResp.GetStatusCode() != 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: int(adjustResp.GetStatusCode()), Msg: adjustResp.GetStatusMsg()})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"product_id": createResp.GetId(),
		"stock":      req.Stock,
		"price":      req.Price,
		"currency":   req.Currency,
	}})
}

func requestUserID(c *gin.Context, fallback int64) int64 {
	if userID := getUserID(c); userID != 0 {
		return userID
	}
	return fallback
}

func toCheckoutItems(items []tradeItem) []*checkoutv1.CheckoutItem {
	result := make([]*checkoutv1.CheckoutItem, 0, len(items))
	for _, item := range items {
		result = append(result, &checkoutv1.CheckoutItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		})
	}
	return result
}

func toCheckoutAddress(addr tradeAddress) *checkoutv1.Address {
	return &checkoutv1.Address{
		ReceiverName: addr.ReceiverName,
		Phone:        addr.Phone,
		Province:     addr.Province,
		City:         addr.City,
		District:     addr.District,
		Street:       addr.Street,
		ZipCode:      addr.ZipCode,
	}
}


