package grpc

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	couponv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/coupon/v1"
)

type CouponHandler struct {
	listUserCouponsUC *usecase.ListUserCouponsUseCase
	evaluateUC        *usecase.EvaluateOrderCouponsUseCase
	reserveUC         *usecase.ReserveCouponUseCase
	commitUC          *usecase.CommitCouponUseCase
	releaseUC         *usecase.ReleaseCouponUseCase
	refundUC          *usecase.RefundCouponUseCase
	issueUC           *usecase.IssueCouponUseCase
}

func NewCouponHandler(
	listUserCouponsUC *usecase.ListUserCouponsUseCase,
	evaluateUC *usecase.EvaluateOrderCouponsUseCase,
	reserveUC *usecase.ReserveCouponUseCase,
	commitUC *usecase.CommitCouponUseCase,
	releaseUC *usecase.ReleaseCouponUseCase,
	refundUC *usecase.RefundCouponUseCase,
	issueUC *usecase.IssueCouponUseCase,
) *CouponHandler {
	return &CouponHandler{
		listUserCouponsUC: listUserCouponsUC,
		evaluateUC:        evaluateUC,
		reserveUC:         reserveUC,
		commitUC:          commitUC,
		releaseUC:         releaseUC,
		refundUC:          refundUC,
		issueUC:           issueUC,
	}
}

// ListUserCoupons 查询用户优惠券列表
func (h *CouponHandler) ListUserCoupons(ctx context.Context, req *couponv1.ListUserCouponsReq) (*couponv1.ListUserCouponsResp, error) {
	output, err := h.listUserCouponsUC.Execute(ctx, usecase.ListUserCouponsInput{
		UserID:   req.GetUserId(),
		Status:   domain.CouponStatus(req.GetStatus()),
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}

	coupons := make([]*couponv1.UserCoupon, 0, len(output.Coupons))
	for _, c := range output.Coupons {
		coupons = append(coupons, domainToProto(c))
	}

	return &couponv1.ListUserCouponsResp{
		Coupons: coupons,
		Total:   output.Total,
	}, nil
}

// ListAvailableCoupons 查询订单可用优惠券（结算页）
func (h *CouponHandler) ListAvailableCoupons(ctx context.Context, req *couponv1.ListAvailableCouponsReq) (*couponv1.ListAvailableCouponsResp, error) {
	items := make([]domain.OrderItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		items = append(items, domain.OrderItem{
			ProductID:  item.GetProductId(),
			CategoryID: item.GetCategoryId(),
			Subtotal:   item.GetPrice() * int64(item.GetQuantity()),
		})
	}

	output, err := h.evaluateUC.Execute(ctx, usecase.EvaluateOrderCouponsInput{
		UserID: req.GetUserId(),
		Items:  items,
	})
	if err != nil {
		return nil, err
	}

	// 只返回可用的券
	coupons := make([]*couponv1.UserCoupon, 0)
	for _, evaluated := range output.Coupons {
		if evaluated.Usable {
			coupons = append(coupons, domainToProto(evaluated.Coupon))
		}
	}

	return &couponv1.ListAvailableCouponsResp{
		Coupons: coupons,
	}, nil
}

// ReserveCoupon 预扣优惠券（创建订单时锁定）
func (h *CouponHandler) ReserveCoupon(ctx context.Context, req *couponv1.ReserveCouponReq) (*couponv1.ReserveCouponResp, error) {
	output, err := h.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    req.GetUserId(),
		CouponIDs: req.GetUserCouponIds(),
		OrderID:   req.GetOrderId(),
	})
	if err != nil {
		return &couponv1.ReserveCouponResp{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &couponv1.ReserveCouponResp{
		Success:  output.Success,
		Message:  reserveCouponMessage(output),
		Failures: toCouponFailures(output.Failures),
	}, nil
}

// CommitCoupon 确认核销（支付成功）
func (h *CouponHandler) CommitCoupon(ctx context.Context, req *couponv1.CommitCouponReq) (*couponv1.CommitCouponResp, error) {
	err := h.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: req.GetOrderId(),
	})
	if err != nil {
		return &couponv1.CommitCouponResp{Success: false}, err
	}

	return &couponv1.CommitCouponResp{Success: true}, nil
}

// ReleaseCoupon 释放预扣（订单取消）
func (h *CouponHandler) ReleaseCoupon(ctx context.Context, req *couponv1.ReleaseCouponReq) (*couponv1.ReleaseCouponResp, error) {
	err := h.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{
		OrderID: req.GetOrderId(),
	})
	if err != nil {
		return &couponv1.ReleaseCouponResp{Success: false}, err
	}

	return &couponv1.ReleaseCouponResp{Success: true}, nil
}

// RefundCoupon 退还优惠券（订单退款）
func (h *CouponHandler) RefundCoupon(ctx context.Context, req *couponv1.RefundCouponReq) (*couponv1.RefundCouponResp, error) {
	err := h.refundUC.Execute(ctx, usecase.RefundCouponInput{
		OrderID: req.GetOrderId(),
	})
	if err != nil {
		return &couponv1.RefundCouponResp{Success: false}, err
	}

	return &couponv1.RefundCouponResp{Success: true}, nil
}

// IssueCoupon 发放优惠券
func (h *CouponHandler) IssueCoupon(ctx context.Context, req *couponv1.IssueCouponReq) (*couponv1.IssueCouponResp, error) {
	output, err := h.issueUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      req.GetUserId(),
		TemplateID:  req.GetTemplateId(),
		OperationID: req.GetOperationId(),
	})
	if err != nil {
		return &couponv1.IssueCouponResp{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &couponv1.IssueCouponResp{
		Success:      true,
		UserCouponId: output.CouponID,
		Message:      "issued successfully",
	}, nil
}

func reserveCouponMessage(output *usecase.ReserveCouponOutput) string {
	if output == nil {
		return ""
	}
	if output.Success {
		return "reserved successfully"
	}
	if len(output.Failures) > 0 {
		return "reserve coupon failed"
	}
	return ""
}

func toCouponFailures(failures []usecase.ReserveCouponFailure) []*couponv1.CouponFailure {
	result := make([]*couponv1.CouponFailure, 0, len(failures))
	for _, failure := range failures {
		result = append(result, &couponv1.CouponFailure{
			UserCouponId: failure.CouponID,
			Reason:       failure.Reason,
		})
	}
	return result
}

// CreateCouponTemplate 创建优惠券模板（暂不实现，预留接口）
func (h *CouponHandler) CreateCouponTemplate(ctx context.Context, req *couponv1.CreateCouponTemplateReq) (*couponv1.CreateCouponTemplateResp, error) {
	// TODO: 实现创建优惠券模板的UseCase
	return nil, errors.New("not implemented yet")
}

// domainToProto 转换domain模型到proto
func domainToProto(c *domain.Coupon) *couponv1.UserCoupon {
	pb := &couponv1.UserCoupon{
		Id:             c.ID,
		UserId:         c.UserID,
		TemplateId:     c.TemplateID,
		Status:         couponv1.UserCouponStatus(c.Status),
		ValidStartTime: c.ValidStartTime.Unix(),
		ValidEndTime:   c.ValidEndTime.Unix(),
		CreatedAt:      c.CreatedAt.Unix(),
	}

	if c.OrderID != 0 {
		pb.LockedOrderId = c.OrderID
		pb.UsedOrderId = c.OrderID
	}

	if !c.UsedAt.IsZero() {
		pb.UsedAt = c.UsedAt.Unix()
	}

	// 转换模板信息（如果有）
	if c.Template != nil {
		pb.Template = &couponv1.CouponTemplate{
			Id:            c.Template.ID,
			Name:          c.Template.Name,
			Type:          couponv1.CouponType(c.Template.Type),
			Threshold:     c.Template.Threshold,
			DiscountValue: c.Template.DiscountValue,
			MaxDiscount:   c.Template.MaxDiscount,
			ValidDays:     c.Template.ValidDays,
			TotalCount:    c.Template.TotalCount,
			IssuedCount:   c.Template.IssuedCount,
			PerUserLimit:  c.Template.PerUserLimit,
			ProductIds:    c.Template.ProductIDs,
			CategoryIds:   c.Template.CategoryIDs,
			Enabled:       c.Template.Enabled,
		}
	}

	return pb
}
