package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	inventoryv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1"
)

type InventoryHandler struct {
	reserveStockUC *usecase.ReserveStockUsecase
	commitStockUC  *usecase.CommitStockUseCase
	releaseStockUC *usecase.ReleaseStockUseCase
	refundStockUC  *usecase.RefundStockUseCase
	repo           domain.InventoryRepository
}

func NewInventoryHandler(
	reserveStockUC *usecase.ReserveStockUsecase,
	commitStockUC *usecase.CommitStockUseCase,
	releaseStockUC *usecase.ReleaseStockUseCase,
	refundStockUC *usecase.RefundStockUseCase,
	repo domain.InventoryRepository,
) *InventoryHandler {
	return &InventoryHandler{
		reserveStockUC: reserveStockUC,
		commitStockUC:  commitStockUC,
		releaseStockUC: releaseStockUC,
		refundStockUC:  refundStockUC,
		repo:           repo,
	}
}

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

func (h *InventoryHandler) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	items := make([]usecase.StockItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = usecase.StockItem{
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
		}
	}

	err := h.reserveStockUC.Execute(ctx, usecase.ReserveStockCommand{
		OperationID: req.OperationId,
		Changes:     items,
		ExpireTime:  req.ExpireTime,
	})
	if err != nil {
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

	return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success"}, nil
}

func (h *InventoryHandler) CommitStock(ctx context.Context, req *inventoryv1.CommitStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	changes := make([]domain.StockChange, len(req.Items))
	for i, item := range req.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductId,
			Quantity:  -item.Quantity,
		}
	}

	err := h.commitStockUC.Execute(ctx, usecase.CommitStockCommand{
		OperationID: req.OperationId,
		Changes:     changes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateOperation) {
			return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success (idempotent)"}, nil
		}
		return &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: err.Error()}, nil
	}

	return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success"}, nil
}

func (h *InventoryHandler) ReleaseStock(ctx context.Context, req *inventoryv1.ReleaseStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := h.releaseStockUC.Execute(ctx, usecase.ReleaseStockCommand{OperationID: req.OperationId})
	if err != nil {
		return &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: err.Error()}, nil
	}

	return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success"}, nil
}

func (h *InventoryHandler) RefundStock(ctx context.Context, req *inventoryv1.RefundStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetOperationId() == "" {
		return nil, errors.New("operation_id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := h.refundStockUC.Execute(ctx, usecase.RefundStockCommand{OperationID: req.OperationId})
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateOperation) {
			return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success (idempotent)"}, nil
		}
		return &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: err.Error()}, nil
	}

	return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success"}, nil
}

func (h *InventoryHandler) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockReq) (*inventoryv1.InventoryOpResp, error) {
	if req.GetReason() == "" {
		return nil, errors.New("reason is required for audit")
	}
	if len(req.GetItems()) == 0 {
		return nil, errors.New("items cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	changes := make([]domain.StockChange, len(req.Items))
	for i, item := range req.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
		}
	}

	operationID := fmt.Sprintf("adjust_%d_%s", time.Now().UnixNano(), req.Reason)
	err := h.repo.AdjustStock(ctx, operationID, req.Reason, changes)
	if err != nil {
		return &inventoryv1.InventoryOpResp{StatusCode: -1, StatusMsg: err.Error()}, nil
	}

	return &inventoryv1.InventoryOpResp{StatusCode: 0, StatusMsg: "success"}, nil
}
