package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type GormInventoryRepository struct {
	db     *gorm.DB
	cache  cache.InventoryCache
	logger logger.LoggerV1
}

func StockKey(productID int64) string {
	return fmt.Sprintf("stock:%d", productID)
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

func (repo *GormInventoryRepository) CommitStock(ctx context.Context, operationID string, changes []domain.StockChange) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operationItems := make([]db.InventoryOperation, len(changes))
		for i, change := range changes {
			operationItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   change.ProductID,
				Type:        "commit",
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
}

func (repo *GormInventoryRepository) RefundStock(ctx context.Context, operationID string) error {
	commitOperationID := operationID
	if strings.HasSuffix(operationID, "refund") {
		commitOperationID = strings.TrimSuffix(operationID, "refund") + "commit"
	}

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

	refundItems := make([]db.InventoryOperation, len(commitRecords))
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
