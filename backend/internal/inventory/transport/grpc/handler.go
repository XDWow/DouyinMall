package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
)

type InventoryHandler struct {
	repo domain.InventoryRepository
}

func NewInventoryHandler(repo domain.InventoryRepository) *InventoryHandler {
	return &InventoryHandler{
		repo: repo,
	}
}

// 查询单个商品库存
func (h *InventoryHandler) GetInventory(ctx context.Context, req *inventoryv1.GetInventoryReq) (*inventoryv1.GetInventoryResp, error) {
	if req.GetProductId() <= 0 {
		return nil, errors.New("product_id must be greater than 0")
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	inventories, err := h.repo.GetInventory(ctx, []int64{req.GetProductId()})
	if err != nil {
		return nil, err
	}

	if len(inventories) == 0 {
		return nil, errors.New("inventory not found")
	}

	inv := inventories[0]
	return &inventoryv1.GetInventoryResp{
		ProductId: inv.ProductID,
	}, nil
}

// BatchGetInventory 批量查询商品库存
func (h *InventoryHandler) BatchGetInventory(ctx context.Context, req *inventoryv1.BatchGetInventoryReq) (*inventoryv1.BatchGetInventoryResp, error) {
	if len(req.GetProductIds()) == 0 {
		return &inventoryv1.BatchGetInventoryResp{Inventories: []*inventoryv1.GetInventoryResp{}}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	inventories, err := h.repo.GetInventory(ctx, req.GetProductIds())
	if err != nil {
		return nil, err
	}

	resp := &inventoryv1.BatchGetInventoryResp{
		Inventories: make([]*inventoryv1.GetInventoryResp, len(inventories)),
	}
	for i, inv := range inventories {
		resp.Inventories[i] = &inventoryv1.GetInventoryResp{
			ProductId:      inv.ProductID,
			AvailableStock: int64(inv.Stock),
			SoldStock:      0,
		}
	}

	return resp, nil
}

// ReserveStock 下单预扣库存（Redis原子操作）
func (h *InventoryHandler) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 转换请求参数
	changes := make([]domain.StockChange, len(req.Items))
	for i, item := range req.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductId,
			Quantity:  -item.Quantity, // 预扣是负数
		}
	}

	// 调用Repository
	err := h.repo.ReserveStock(ctx, req.OperationId, changes, req.ExpireTime)
	if err != nil {
		// 库存不足：返回明细
		var stockErr *domain.InsufficientStockError
		if errors.As(err, &stockErr) {
			insufficient := make([]*inventoryv1.InsufficientItem, len(stockErr.Items))
			for i, item := range stockErr.Items {
				insufficient[i] = &inventoryv1.InsufficientItem{
					ProductId: item.ProductID,
					Requested: int32(item.Requested),
					Available: item.Available,
				}
			}
			return &inventoryv1.InventoryOpResp{
				StatusCode:        -2,
				StatusMsg:         stockErr.Error(),
				InsufficientItems: insufficient,
			}, fmt.Errorf("%w", stockErr)
		}
		return &inventoryv1.InventoryOpResp{
			StatusCode: -1,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &inventoryv1.InventoryOpResp{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// CommitStock 支付成功，确认扣减库存（DB强一致）
func (h *InventoryHandler) CommitStock(ctx context.Context, req *inventoryv1.CommitStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 转换请求参数
	changes := make([]domain.StockChange, len(req.Items))
	for i, item := range req.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductId,
			Quantity:  -item.Quantity, // 扣减是负数
		}
	}

	// 调用Repository
	err := h.repo.CommitStock(ctx, req.OperationId, changes)
	if err != nil {
		// 幂等冲突，返回成功
		if errors.Is(err, domain.ErrDuplicateOperation) {
			return &inventoryv1.InventoryOpResp{
				StatusCode: 0,
				StatusMsg:  "success (idempotent)",
			}, nil
		}
		return &inventoryv1.InventoryOpResp{
			StatusCode: -1,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &inventoryv1.InventoryOpResp{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// ReleaseStock 订单取消，释放预扣库存
func (h *InventoryHandler) ReleaseStock(ctx context.Context, req *inventoryv1.ReleaseStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 调用Repository
	err := h.repo.ReleaseStock(ctx, req.OperationId)
	if err != nil {
		return &inventoryv1.InventoryOpResp{
			StatusCode: -1,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &inventoryv1.InventoryOpResp{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// RefundStock 已售商品退款，恢复库存
// operationID: 原始commit的operationID，从commit记录读取商品信息
func (h *InventoryHandler) RefundStock(ctx context.Context, req *inventoryv1.RefundStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 调用Repository（从commit记录读取商品信息）
	err := h.repo.RefundStock(ctx, req.OperationId)
	if err != nil {
		// 幂等冲突，返回成功
		if errors.Is(err, domain.ErrDuplicateOperation) {
			return &inventoryv1.InventoryOpResp{
				StatusCode: 0,
				StatusMsg:  "success (idempotent)",
			}, nil
		}
		return &inventoryv1.InventoryOpResp{
			StatusCode: -1,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &inventoryv1.InventoryOpResp{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// AdjustStock 人工调整库存（内部管理接口）
func (h *InventoryHandler) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetReason() == "" {
		return nil, errors.New("reason is required for audit")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 转换请求参数
	changes := make([]domain.StockChange, len(req.Items))
	for i, item := range req.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductId,
			Quantity:  item.Quantity, // 带符号的数量
		}
	}

	// 生成唯一操作ID（时间戳+原因摘要，确保幂等）
	operationID := fmt.Sprintf("adjust_%d_%s", time.Now().UnixNano(), req.Reason)

	// 调用Repository
	err := h.repo.AdjustStock(ctx, operationID, req.Reason, changes)
	if err != nil {
		return &inventoryv1.InventoryOpResp{
			StatusCode: -1,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &inventoryv1.InventoryOpResp{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}
