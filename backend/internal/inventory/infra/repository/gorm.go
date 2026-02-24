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

/*
Redis存储结构：

 1. stock:{productID} - String类型，存储商品预库存数量
    如: stock:12345 = "100"

 2. reserve:{reserveID} - Hash类型，存储预扣记录
    如: reserve:前端传uuid = {
    "10001": "-5",   // 负数表示扣减
    "10002": "-3"
    }
    TTL: 35分钟（订单超时30分钟 + 5分钟缓冲）
*/
func StockKey(productID int64) string {
	return fmt.Sprintf("stock:%d", productID)
}

// 幂等预扣
func ReserveKey(reserveID string) string {
	return fmt.Sprintf("reserve:%s", reserveID)
}

// 为了幂等恢复预库存
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
	// 构造 Lua 脚本参数
	keys := []string{ReserveKey(reserveID)}
	args := []interface{}{expireTime}

	for _, change := range changes {
		args = append(args, strconv.FormatInt(change.ProductID, 10), change.Quantity)
	}

	// 执行 Lua 脚本
	result, err := repo.cache.Eval(ctx, reserveStockScript, keys, args...)
	if err != nil {
		return fmt.Errorf("预扣失败: %w", err)
	}

	// Lua 返回 int64 表示状态码，返回 []interface{} 表示库存不足明细
	switch v := result.(type) {
	case int64:
		switch v {
		case 0:
			return nil // 重复操作，幂等返回成功
		case 1:
			return nil // 预扣成功
		case -1:
			return domain.ErrProductNotFound
		default:
			return fmt.Errorf("未知错误: %d", v)
		}
	case []interface{}:
		// Lua table 返回: [productID1, requested1, available1, ...]
		if len(v)%3 != 0 {
			return fmt.Errorf("库存不足，但返回数据异常")
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
		return fmt.Errorf("未知返回类型: %T", result)
	}
}

func (repo *GormInventoryRepository) CommitStock(ctx context.Context, operationID string, changes []domain.StockChange) error {
	// DB 事务：批量插入操作记录 + 扣减库存（不允许部分失败，如果部分失败，设计难度上升，状态从订单级到商品级）
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 批量插入操作记录（幂等：operation_id + product_id 唯一索引）
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
			// 检查是否是唯一索引冲突
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				// Duplicate entry，幂等冲突
				return domain.ErrDuplicateOperation
			}
			// 其他错误
			return fmt.Errorf("插入操作记录失败: %w", err)
		}

		// 2. 通过了幂等校验，保存了记录，批量扣减库存+累加sold（行锁 + 检查库存）
		// 所有商品库存的变化，一个事务，减少了事务开销
		for _, change := range changes {
			absQuantity := -change.Quantity // 转为正数用于检查
			result := tx.Model(&db.Inventory{}).
				Where("product_id = ? AND stock >= ?", change.ProductID, absQuantity).
				Updates(map[string]interface{}{
					"stock": gorm.Expr("stock + ?", change.Quantity), // 扣减库存（change.Quantity是负数）
					"sold":  gorm.Expr("sold + ?", absQuantity),      // 累加已售出
				})

			if result.Error != nil {
				return fmt.Errorf("扣减库存失败: %w", result.Error)
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
	// 本来想着内存很贵，主动删了预扣记录，因为不需要靠他恢复预扣库存了，但是还要幂等啊，反正不大，订单生命周期内都留着吧
	// 不主动删除预扣记录，让其自然过期
	return nil
}

func (repo *GormInventoryRepository) ReleaseStock(ctx context.Context, operationID string) error {
	// 反向构建key，恢复数量
	reserveID := strings.Replace(operationID, "release", "reserve", 1)

	// 订单取消，恢复Redis预库存
	// 幂等机制：使用独立的release幂等key，而不是之前的删除预扣记录实现幂等，那样影响了预扣幂等
	// KEYS[1] = release:{releaseID} (幂等key)
	// KEYS[2] = reserve:{reserveID} (预扣记录)
	keys := []string{ReleaseKey(operationID), ReserveKey(reserveID)}
	args := []interface{}{2100} // 35分钟过期（订单30分钟超时 + 5分钟缓冲）

	result, err := repo.cache.Eval(ctx, releaseStockScript, keys, args...)
	if err != nil {
		return fmt.Errorf("释放预扣失败: %w", err)
	}

	code := result.(int64)
	switch code {
	case 0:
		// 已释放过（幂等）
		return nil
	case 1:
		// 释放成功
		return nil
	case -1:
		// 预扣记录不存在（可能已过期或从未预扣）
		return nil
	default:
		// 由于lua的原子性不具备acid，这里有可能恢复完库存，redis崩了，记录没删除，导致重试过来会多删，选择接受风险，因为概率低，影响有限，
		// 不为极低概率事件，低回报事件增加复杂度，定时任务去修复吧
		return fmt.Errorf("未知错误: %d", code)
	}
}

// 订单退款，恢复DB库存和Redis预库存
// operationID: 本次refund操作的ID（如order_123_refund），用于幂等检查和插入记录
func (repo *GormInventoryRepository) RefundStock(ctx context.Context, operationID string) error {
	// 1. 从refund的operationID推导出commit的operationID（order_123_refund -> order_123_commit）
	commitOperationID := strings.Replace(operationID, "refund", "commit", 1)

	// 2. 从commit记录查询商品信息
	var commitRecords []db.InventoryOperation
	err := repo.db.WithContext(ctx).
		Where("operation_id = ? AND type = ?", commitOperationID, "commit").
		Find(&commitRecords).Error
	if err != nil {
		return fmt.Errorf("查询commit记录失败: %w", err)
	}
	if len(commitRecords) == 0 {
		// 没有commit记录，说明从未提交过，无需退款
		repo.logger.Info("RefundStock: commit记录不存在，跳过",
			logger.String("commitOperationID", commitOperationID))
		return nil
	}

	refundItems := make([]db.InventoryOperation, len(commitRecords))
	err = repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 3. 批量插入退款操作记录（幂等：用refund operationID）
		for i, record := range commitRecords {
			refundItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   record.ProductID,
				Type:        "refund",
				Quantity:    -record.Quantity, // 取反：commit扣了多少，refund就恢复多少，正数
			}
		}
		if err := tx.Create(&refundItems).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return domain.ErrDuplicateOperation
			}
			return fmt.Errorf("插入退款记录失败: %w", err)
		}

		// 4. 通过幂等校验，恢复DB库存+减少sold
		for _, record := range refundItems {
			err := tx.Model(&db.Inventory{}).
				Where("product_id = ?", record.ProductID).
				Updates(map[string]interface{}{
					"stock": gorm.Expr("stock + ?", record.Quantity), // 恢复库存（record.Quantity是正数）
					"sold":  gorm.Expr("sold - ?", record.Quantity),  // 减少已售出
				}).Error
			if err != nil {
				return fmt.Errorf("恢复库存失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 5. 恢复 Redis 预库存（容错执行：DB已成功，Redis失败不影响业务正确性）
	// 直接用db中的记录数量恢复，无需再查Redis预扣记录，之前也太脑残了，还去查预扣记录，现在少查一次，而且是数据源
	for _, record := range refundItems {
		_, err := repo.cache.IncrBy(ctx, StockKey(record.ProductID), record.Quantity)
		if err != nil {
			repo.logger.Warn("RefundStock: Redis预库存恢复失败（定时任务将兜底修复）",
				logger.Int64("productID", record.ProductID),
				logger.Int32("quantity", -record.Quantity),
				logger.Error(err))
			// 容忍错误，继续处理其他商品
		}
	}

	return nil
}

func (repo *GormInventoryRepository) AdjustStock(ctx context.Context, operationID string, reason string, changes []domain.StockChange) error {
	// 1. DB事务：批量插入调整记录 + 更新库存
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 批量插入调整记录（幂等）
		operationItems := make([]db.InventoryOperation, len(changes))
		for i, change := range changes {
			operationItems[i] = db.InventoryOperation{
				OperationID: operationID,
				ProductID:   change.ProductID,
				Type:        "adjust",
				Reason:      reason, // 调整原因（审计）
				Quantity:    change.Quantity,
			}
		}

		if err := tx.Create(&operationItems).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return domain.ErrDuplicateOperation
			}
			return fmt.Errorf("插入调整记录失败: %w", err)
		}

		// 调整库存
		for _, change := range changes {
			result := tx.Model(&db.Inventory{}).Where("product_id = ?", change.ProductID).
				Update("stock", gorm.Expr("stock + ?", change.Quantity))
			if result.Error != nil {
				return fmt.Errorf("调整库存失败: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("商品 %d 不存在", change.ProductID)
			}
		}

		return nil
	})

	// DB事务失败，直接返回
	if err != nil {
		return err
	}

	// 2. 容错更新Redis预库存（IncrBy增量更新，之前还直接数据库数据覆盖，其实他两库存数量不同）
	// 思路：DB成功=业务成功，缓存失败仅告警，定时任务兜底
	for _, change := range changes {
		_, err := repo.cache.IncrBy(ctx, StockKey(change.ProductID), change.Quantity)
		if err != nil {
			repo.logger.Warn("AdjustStock: Redis增量更新失败（定时任务将兜底修复）",
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
