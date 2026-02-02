package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ReserveStockUc struct {
	repo domain.InventoryRepository
	l    logger.LoggerV1
}

func NewReserveStockUc(repo domain.InventoryRepository, l logger.LoggerV1) *ReserveStockUc {
	return &ReserveStockUc{repo, l}
}

func (uc *ReserveStockUc) Execute(ctx context.Context, cmd ReserveStockCommand) error {
	if cmd.OperationID == "" {
		return errors.New("OperationID is empty")
	}
	changes := []domain.StockChange{}
	for _, item := range cmd.Changes {
		changes = append(changes, domain.StockChange{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}
	return uc.repo.ReserveStock(ctx, cmd.OperationID, changes, cmd.ExpireTime)
}

/*
Redis 是键值型存储，不适合承载需要人工排查的异常状态。
预扣记录属于临时并发控制数据，应设置 TTL 自动清理，
TTL 略大于订单过期时间，以覆盖正常业务延迟，
避免永久占用内存和库存泄漏
*/
type ReserveStockCommand struct {
	OperationID string
	Changes     []StockItem
	ExpireTime  int64
}

type StockItem struct {
	ProductID int64
	Quantity  int32
}
