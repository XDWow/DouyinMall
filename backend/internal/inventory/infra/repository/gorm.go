package repository

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

//go:embed lua/reserve_stock.lua
var reserveStockScript string

//go:embed lua/release_stock.lua
var releaseStockScript string

type GormInventoryRepository struct {
	db     *gorm.DB
	cache  cache.InventoryCache
	logger logger.LoggerV1
}

func StockKey(productID int64) string {
	return fmt.Sprintf("stock:%d", productID)
}

func ReserveKey(reserveID string) string {
	return fmt.Sprintf("reserve:%s", reserveID)
}

func ReleaseKey(releaseID string) string {
	return fmt.Sprintf("release:%s", releaseID)
}

func (repo *GormInventoryRepository) GetInventory(ctx context.Context, productID []int64) ([]domain.Inventory, error) {
	var model []db.Inventory
	err := repo.db.WithContext(ctx).Find(&model, "product_id in (?)", productID).Error
	if err != nil {
		return nil, err
	}
	result := make([]domain.Inventory, len(model))
	for i, v := range model {
		result[i] = domain.Inventory{
			ProductID: v.ProductID,
			Stock:     v.Stock,
		}
	}
	return result, nil
}

func (repo *GormInventoryRepository) ReserveStock(ctx context.Context, reserveID string, changes []domain.StockChange, expireTime int64) error {
	keys := []string{ReserveKey(reserveID)}
	args := []interface{}{expireTime}

	for _, change := range changes {
		args = append(args, strconv.FormatInt(change.ProductID, 10), change.Quantity)
	}

	result, err := repo.cache.Eval(ctx, reserveStockScript, keys, args...)
	if err != nil {
		return fmt.Errorf("reserve stock failed: %w", err)
	}

	switch v := result.(type) {
	case int64:
		switch v {
		case 0, 1:
			return nil
		case -1:
			return domain.ErrProductNotFound
		default:
			return fmt.Errorf("unknown reserve result: %d", v)
		}
	case []interface{}:
		if len(v)%3 != 0 {
			return fmt.Errorf("invalid insufficient stock payload")
		}
		items := make([]domain.InsufficientStockItem, 0, len(v)/3)
		for i := 0; i < len(v); i += 3 {
			pid, _ := strconv.ParseInt(v[i].(string), 10, 64)
			req := v[i+1].(int64)
			avail := v[i+2].(int64)
			items = append(items, domain.InsufficientStockItem{
				ProductID: pid,
				Requested: req,
				Available: avail,
			})
		}
		return &domain.InsufficientStockError{Items: items}
	default:
		return fmt.Errorf("unknown reserve result type: %T", result)
	}
}

func (repo *GormInventoryRepository) CommitStock(ctx context.Context, operationID string, changes []domain.StockChange) error {
	commitType := "commit"
	if !repo.hasReserveRecord(ctx, operationID) {
		commitType = "commit_direct"
	}

	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operationItems := make([]db.InventoryOperation, len(changes))
		for i, change := range changes {
			operationItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   change.ProductID,
				Type:        commitType,
				Quantity:    change.Quantity,
			}
		}

		if err := tx.Create(&operationItems).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return domain.ErrDuplicateOperation
			}
			return fmt.Errorf("insert inventory operation failed: %w", err)
		}

		for _, change := range changes {
			absQuantity := -change.Quantity
			result := tx.Model(&db.Inventory{}).
				Where("product_id = ? AND stock >= ?", change.ProductID, absQuantity).
				Updates(map[string]interface{}{
					"stock": gorm.Expr("stock + ?", change.Quantity),
					"sold":  gorm.Expr("sold + ?", absQuantity),
				})

			if result.Error != nil {
				return fmt.Errorf("commit stock failed: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return domain.ErrInsufficientStock
			}
		}

		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func (repo *GormInventoryRepository) ReleaseStock(ctx context.Context, operationID string) error {
	for _, reserveID := range reserveIDCandidates(operationID, "release", "reserve") {
		code, err := repo.evalReleaseStock(ctx, operationID, reserveID)
		if err != nil {
			return err
		}
		if code == 0 || code == 1 {
			return nil
		}
	}
	return nil
}

func (repo *GormInventoryRepository) RefundStock(ctx context.Context, operationID string) error {
	commitOperationID := strings.Replace(operationID, "refund", "commit", 1)

	var commitRecords []db.InventoryOperation
	err := repo.db.WithContext(ctx).
		Where("operation_id = ? AND type IN ?", commitOperationID, []string{"commit", "commit_direct"}).
		Find(&commitRecords).Error
	if err != nil {
		return fmt.Errorf("query commit records failed: %w", err)
	}
	if len(commitRecords) == 0 {
		repo.logger.Info("RefundStock: commit record not found",
			logger.String("commitOperationID", commitOperationID))
		return nil
	}

	restoreRedis := false
	for _, record := range commitRecords {
		if record.Type == "commit" {
			restoreRedis = true
			break
		}
	}

	refundItems := make([]db.InventoryOperation, len(commitRecords))
	err = repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, record := range commitRecords {
			refundItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   record.ProductID,
				Type:        "refund",
				Quantity:    -record.Quantity,
			}
		}
		if err := tx.Create(&refundItems).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return domain.ErrDuplicateOperation
			}
			return fmt.Errorf("insert refund operation failed: %w", err)
		}

		for _, record := range refundItems {
			err := tx.Model(&db.Inventory{}).
				Where("product_id = ?", record.ProductID).
				Updates(map[string]interface{}{
					"stock": gorm.Expr("stock + ?", record.Quantity),
					"sold":  gorm.Expr("sold - ?", record.Quantity),
				}).Error
			if err != nil {
				return fmt.Errorf("restore stock failed: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !restoreRedis {
		return nil
	}

	for _, record := range refundItems {
		_, err := repo.cache.IncrBy(ctx, StockKey(record.ProductID), record.Quantity)
		if err != nil {
			repo.logger.Warn("RefundStock: restore redis stock failed",
				logger.Int64("productID", record.ProductID),
				logger.Int32("quantity", record.Quantity),
				logger.Error(err))
		}
	}

	return nil
}

func (repo *GormInventoryRepository) AdjustStock(ctx context.Context, operationID string, reason string, changes []domain.StockChange) error {
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operationItems := make([]db.InventoryOperation, len(changes))
		for i, change := range changes {
			operationItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   change.ProductID,
				Type:        "adjust",
				Reason:      reason,
				Quantity:    change.Quantity,
			}
		}

		if err := tx.Create(&operationItems).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return domain.ErrDuplicateOperation
			}
			return fmt.Errorf("insert adjust operation failed: %w", err)
		}

		for _, change := range changes {
			result := tx.Model(&db.Inventory{}).Where("product_id = ?", change.ProductID).
				Update("stock", gorm.Expr("stock + ?", change.Quantity))
			if result.Error != nil {
				return fmt.Errorf("adjust stock failed: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				if change.Quantity <= 0 {
					return fmt.Errorf("product %d not found", change.ProductID)
				}

				if err := tx.Create(&db.Inventory{
					ProductID: change.ProductID,
					Stock:     int64(change.Quantity),
					Sold:      0,
				}).Error; err != nil {
					var mysqlErr *mysql.MySQLError
					if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
						retry := tx.Model(&db.Inventory{}).Where("product_id = ?", change.ProductID).
							Update("stock", gorm.Expr("stock + ?", change.Quantity))
						if retry.Error != nil {
							return fmt.Errorf("retry adjust stock failed: %w", retry.Error)
						}
						if retry.RowsAffected == 0 {
							return fmt.Errorf("product %d not found", change.ProductID)
						}
						continue
					}
					return fmt.Errorf("create inventory failed: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	for _, change := range changes {
		_, err := repo.cache.IncrBy(ctx, StockKey(change.ProductID), change.Quantity)
		if err != nil {
			repo.logger.Warn("AdjustStock: update redis stock failed",
				logger.Int64("productID", change.ProductID),
				logger.Int32("delta", change.Quantity),
				logger.Error(err))
		}
	}

	return nil
}

func NewGormInventoryRepository(db *gorm.DB, cache cache.InventoryCache, l logger.LoggerV1) domain.InventoryRepository {
	return &GormInventoryRepository{
		db:     db,
		cache:  cache,
		logger: l,
	}
}

func (repo *GormInventoryRepository) hasReserveRecord(ctx context.Context, operationID string) bool {
	for _, reserveID := range reserveIDCandidates(operationID, "commit", "reserve") {
		if _, err := repo.cache.Get(ctx, ReserveKey(reserveID)); err == nil {
			return true
		}
	}
	return false
}

func (repo *GormInventoryRepository) evalReleaseStock(ctx context.Context, operationID, reserveID string) (int64, error) {
	result, err := repo.cache.Eval(ctx, releaseStockScript, []string{
		ReleaseKey(operationID),
		ReserveKey(reserveID),
	}, 2100)
	if err != nil {
		return 0, fmt.Errorf("release stock failed: %w", err)
	}

	code, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unknown release result type: %T", result)
	}
	switch code {
	case 0, 1, -1:
		return code, nil
	default:
		return 0, fmt.Errorf("unknown release result: %d", code)
	}
}

func reserveIDCandidates(operationID, fromAction, toAction string) []string {
	candidates := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		candidates = append(candidates, id)
	}

	add(strings.Replace(operationID, fromAction, toAction, 1))

	if orderID, ok := orderIDFromOperation(operationID); ok {
		add(fmt.Sprintf("%d:%s", orderID, toAction))
	}

	return candidates
}

func orderIDFromOperation(operationID string) (int64, bool) {
	if strings.HasPrefix(operationID, "order_") {
		rest := strings.TrimPrefix(operationID, "order_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) > 0 {
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil {
				return id, true
			}
		}
	}

	parts := strings.SplitN(operationID, ":", 2)
	if len(parts) == 2 {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err == nil {
			return id, true
		}
	}

	return 0, false
}


