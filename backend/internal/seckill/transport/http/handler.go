package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	getActivityUC *usecase.GetActivityUseCase
	submitUC      *usecase.SubmitUseCase
	getResultUC   *usecase.GetResultUseCase
}

func NewHandler(
	getActivityUC *usecase.GetActivityUseCase,
	submitUC *usecase.SubmitUseCase,
	getResultUC *usecase.GetResultUseCase,
) *Handler {
	return &Handler{
		getActivityUC: getActivityUC,
		submitUC:      submitUC,
		getResultUC:   getResultUC,
	}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginx.Result{Code: 0, Msg: "ok"})
	})

	engine.GET("/api/seckill/activities/:activityId", h.GetActivity)

	group := engine.Group("/api/seckill", ginx.RequireGatewayUser())
	group.POST("/submit", h.Submit)
	group.GET("/result", h.GetResult)
}

func (h *Handler) GetActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("activityId"), 10, 64)
	if err != nil || activityID <= 0 {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid activity id"})
		return
	}

	activity, err := h.getActivityUC.Execute(c.Request.Context(), activityID)
	if err != nil {
		writeSeckillError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"activity": gin.H{
			"activity_id":     activity.ID,
			"activity_name":   activity.ActivityName,
			"product_id":      activity.ProductID,
			"sku_id":          activity.SKUID,
			"seckill_price":   activity.SeckillPrice,
			"total_stock":     activity.TotalStock,
			"available_stock": activity.AvailableStock,
			"start_time":      activity.StartTime.Unix(),
			"end_time":        activity.EndTime.Unix(),
			"status":          activity.Status,
			"limit_per_user":  activity.LimitPerUser,
		},
	}})
}

type submitRequest struct {
	ActivityID int64 `json:"activity_id" binding:"required"`
}

func (h *Handler) Submit(c *gin.Context) {
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "invalid submit request: " + err.Error()})
		return
	}

	result, err := h.submitUC.Execute(c.Request.Context(), usecase.SubmitCmd{
		ActivityID: req.ActivityID,
		UserID:     ginx.GatewayUserID(c),
	})
	if err != nil && result == nil {
		writeSeckillError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"request_no":  result.RequestNo,
		"status":      result.Status,
		"order_id":    result.OrderID,
		"fail_reason": result.FailReason,
	}})
}

func (h *Handler) GetResult(c *gin.Context) {
	requestNo := c.Query("request_no")
	if requestNo == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "request_no is required"})
		return
	}

	result, err := h.getResultUC.ExecuteForUser(c.Request.Context(), requestNo, ginx.GatewayUserID(c))
	if err != nil {
		writeSeckillError(c, err)
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"request_no":  result.RequestNo,
		"status":      result.Status,
		"order_id":    result.OrderID,
		"fail_reason": result.FailReason,
	}})
}

func writeSeckillError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5

	switch {
	case errors.Is(err, domain.ErrActivityNotFound), errors.Is(err, domain.ErrRequestNotFound):
		status = http.StatusNotFound
		code = 4
	case errors.Is(err, domain.ErrActivityNotStarted),
		errors.Is(err, domain.ErrActivityEnded),
		errors.Is(err, domain.ErrActivityOffline),
		errors.Is(err, domain.ErrDuplicateSeckill),
		errors.Is(err, domain.ErrOutOfStock):
		status = http.StatusBadRequest
		code = 4
	}

	c.JSON(status, ginx.Result{
		Code: code,
		Msg:  err.Error(),
	})
}
