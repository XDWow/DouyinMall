package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

// ErrRecordNotFound 已移到domain层
// var ErrRecordNotFound = gorm.ErrRecordNotFound

const (
	orderTTL         = 30 * time.Second
	userOrderListTTL = 30 * time.Second
	pageLimit        = 10
)

// 缓存场景分析：
// 一致性高 + 低并发 → 直接 DB
// 一致性高 + 高并发 → 删缓存(cache-aside，立刻拿到最新数据) + 短TTL（删除缓存失败了的兜底，最多延迟一会自动过期）（核心：不信任地使用缓存）
// 一致性低 + 高并发 → 维护缓存
// 一致性低 + 低并发 → 甚至不需要 Redis

// 订单系统采用旁路缓存（Cache Aside）
// DB 为唯一事实源，缓存仅用于加速高并发读
// 通过 TTL 兜底，允许几十秒的展示滞后，但保证最终一致性
// 不允许缓存参与业务状态演进
type orderRepository struct {
	db    *gorm.DB
	cache cache.OrderCache
	log   logger.LoggerV1
}

func NewOrderRepository(db *gorm.DB, cache cache.OrderCache, log logger.LoggerV1) domain.OrderRepository {
	return &orderRepository{
		db:    db,
		cache: cache,
		log:   log,
	}
}

func orderKey(orderID int64) string {
	return fmt.Sprintf("order:%d", orderID)
}

func userOrderListKey(userID int64) string {
	return fmt.Sprintf("order:user:%d:ids", userID)
}

func (repo *orderRepository) Save(ctx context.Context, order *domain.Order) error {
	orderModel := toOrderModel(order)
	// GORM会自动设置OrderItems的OrderID外键
	if err := repo.db.WithContext(ctx).Create(orderModel).Error; err != nil {
		return err
	}
	// 回写生成的ID到domain对象
	order.ID = orderModel.ID

	// 写缓存，失败了打日志就行，大不了不加速了而已
	data, err := json.Marshal(orderModel)
	if err != nil {
		repo.log.Warn("订单序列化失败，无法写入缓存", logger.Error(err), logger.Int64("orderID", order.ID))
		return nil
	}
	if err := repo.cache.Set(ctx, orderKey(orderModel.ID), string(data), orderTTL); err != nil {
		repo.log.Warn("写入订单缓存失败", logger.Error(err), logger.Int64("orderID", order.ID))
	}

	members := map[string]float64{
		strconv.FormatInt(orderModel.ID, 10): float64(orderModel.CreatedAt.UnixMilli()),
	}
	// 使用ZAddWithLimit保持固定大小
	if err := repo.cache.ZAddWithLimit(ctx, userOrderListKey(orderModel.UserID), members, pageLimit, userOrderListTTL); err != nil {
		repo.log.Warn("写入用户订单列表缓存失败", logger.Error(err), logger.Int64("userID", orderModel.UserID))
	}

	return nil
}

func (repo *orderRepository) FindByID(ctx context.Context, orderID int64) (domain.Order, error) {
	var orderModel db.OrderModel
	err := repo.db.WithContext(ctx).
		Preload("Items"). // 预加载订单项
		Where("id = ?", orderID).
		First(&orderModel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Order{}, domain.ErrRecordNotFound
		}
		return domain.Order{}, err
	}
	return *toDomainOrder(&orderModel), nil
}

func (repo *orderRepository) UpdateStatus(ctx context.Context, order *domain.Order) error {
	res := repo.db.WithContext(ctx).
		Model(&db.OrderModel{}).
		Where("id = ? && status = ?", order.ID, domain.OrderStatusCreated.AsUint8()).
		Update("status", order.Status.AsUint8())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	// 这里删除缓存失败会导致数据不一致，只能依赖短TTL兜底，无能的丈夫
	err := repo.cache.Del(ctx, orderKey(order.ID))
	if err != nil {
		repo.log.Warn("删除订单缓存失败", logger.Error(err), logger.Int64("orderID", order.ID))
	}
	return nil
}

// 现在的场景是：批量取消订单
func (repo *orderRepository) BatchUpdateStatus(ctx context.Context, orderIDs []int64, fromStatus, toStatus domain.OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}

	res := repo.db.WithContext(ctx).
		Model(&db.OrderModel{}).
		Where("id IN ? AND status = ?", orderIDs, fromStatus.AsUint8()).
		Update("status", toStatus.AsUint8())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}

	keys := make([]string, len(orderIDs))
	for i, id := range orderIDs {
		keys[i] = orderKey(id)
	}
	repo.cache.Del(ctx, keys...)

	return nil
}

func (repo *orderRepository) ListByUserID(
	ctx context.Context,
	userID int64,
	cursor int64,
	limit int,
) (orders []*domain.Order, nextCursor int64, err error) {
	// 只缓存第一页（cursor=0时的热点数据）
	if cursor == 0 && limit <= pageLimit {
		// 1. 从ZSet获取orderID列表
		idStrs, err := repo.cache.ZRange(ctx, userOrderListKey(userID), 0, int64(limit-1), true)
		if err != nil {
			repo.log.Warn("ZRange失败，fallback到DB", logger.Error(err), logger.Int64("userID", userID))
		} else if len(idStrs) > 0 {
			orderIDs := make([]int64, 0, len(idStrs))
			for _, idStr := range idStrs {
				if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
					orderIDs = append(orderIDs, id)
				}
			}
			if len(orderIDs) > 0 {
				// 2. 批量MGet查询缓存
				keys := make([]string, len(orderIDs))
				for i, id := range orderIDs {
					keys[i] = orderKey(id)
				}

				dataList, err := repo.cache.MGet(ctx, keys...)
				if err != nil {
					repo.log.Warn("MGet失败，fallback到DB", logger.Error(err), logger.Int64("userID", userID))
				} else {
					// 3. 分离命中和未命中的orderIDs
					orders := make([]*domain.Order, 0, len(orderIDs))
					missIDs := make([]int64, 0)
					missIndexes := make(map[int64]int) // orderID -> orders中的位置

					for i, data := range dataList {
						if data == nil {
							// 缓存未命中
							missIDs = append(missIDs, orderIDs[i])
							missIndexes[orderIDs[i]] = len(orders)
							orders = append(orders, nil) // 占位
						} else {
							var order domain.Order
							if err := json.Unmarshal([]byte(*data), &order); err != nil {
								repo.log.Warn("反序列化订单失败", logger.Error(err), logger.Int64("orderID", orderIDs[i]))
								// 反序列化失败，当作miss处理
								missIDs = append(missIDs, orderIDs[i])
								missIndexes[orderIDs[i]] = len(orders)
								orders = append(orders, nil)
							} else {
								orders = append(orders, &order)
							}
						}
					}

					// 4. 去DB补充查询未命中的订单
					if len(missIDs) > 0 {
						var missModels []db.OrderModel
						err := repo.db.WithContext(ctx).
							Where("id IN ?", missIDs).
							Find(&missModels).Error
						if err != nil {
							repo.log.Error("DB查询miss订单失败", logger.Error(err))
							return nil, 0, err
						}

						// 5. 回填到结果 orders，并写回缓存

						for _, model := range missModels {
							order := toDomainOrder(&model)

							if idx, ok := missIndexes[model.ID]; ok {
								orders[idx] = order
							}

							// 写回缓存
							if data, e := json.Marshal(&order); e == nil {
								repo.cache.Set(ctx, orderKey(order.ID), string(data), orderTTL)
							}
						}
					}

					// 6. 过滤掉仍然为空的占位（DB中也不存在，说明缓存list有脏数据）
					validOrders := make([]*domain.Order, 0, len(orders))
					hasInvalidData := false
					for _, o := range orders {
						if o.ID != 0 {
							validOrders = append(validOrders, o)
						} else {
							hasInvalidData = true
						}
					}

					// 数据库没有（真相），但是list竟然有，那就是脏数据，删除list缓存
					if hasInvalidData {
						repo.log.Warn("缓存list存在脏数据，删除list缓存", logger.Int64("userID", userID))
						repo.cache.Del(ctx, userOrderListKey(userID))
					}

					// 计算nextCursor
					if len(validOrders) < limit { // 没有下一页了
						return validOrders, 0, nil
					}
					nextCursor = validOrders[len(validOrders)-1].ID
					return validOrders, nextCursor, nil
				}
			}
		}
	}

	// 非首页，不走缓存，直接DB查询
	var models []db.OrderModel
	// Limit(limit+1)来判断是否还有下一页
	err = repo.db.WithContext(ctx).
		Where("user_id = ? && id < ?", userID, cursor).
		Order("id DESC").
		Limit(limit + 1).
		Find(&models).Error
	if err != nil {
		return nil, cursor, err
	}

	// 如果查询结果超过limit，说明有下一页
	hasMore := len(models) > limit
	if hasMore {
		models = models[:limit] // 只返回limit条
	}

	orders = make([]*domain.Order, len(models))
	for i, model := range models {
		orders[i] = toDomainOrder(&model)
	}

	if hasMore {
		nextCursor = orders[len(orders)-1].ID
	} else {
		nextCursor = 0
	}

	return orders, nextCursor, nil
}

func (repo *orderRepository) FindExpiredOrders(ctx context.Context, limit int) ([]*domain.Order, error) {
	var models []db.OrderModel
	query := repo.db.WithContext(ctx).
		Where("status = ? AND expired_at < ?", domain.OrderStatusCreated, time.Now())

	// limit > 0 才限制数量，否则查询所有
	if limit > 0 {
		query = query.Limit(limit)
	}

	res := query.Find(&models)
	if res.Error != nil {
		return nil, res.Error
	}

	orders := make([]*domain.Order, len(models))
	for i, model := range models {
		orders[i] = toDomainOrder(&model)
	}
	return orders, nil
}

// 列出某用户某状态的订单列表，这个不是热点数据吧，不缓存了
func (repo *orderRepository) ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*domain.Order, error) {
	var models []db.OrderModel
	err := repo.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, status).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	orders := make([]*domain.Order, len(models))
	for i, model := range models {
		orders[i] = toDomainOrder(&model)
	}
	return orders, err
}

func toOrderModel(order *domain.Order) *db.OrderModel {
	m := &db.OrderModel{
		ID:        order.ID,
		UserID:    order.UserID,
		Phone:     order.Phone,
		Status:    uint8(order.Status),
		Currency:  order.Amt.Currency,
		Total:     order.Amt.Total,
		Street:    order.Addr.Street,
		City:      order.Addr.City,
		State:     order.Addr.State,
		Country:   order.Addr.Country,
		ZipCode:   order.Addr.Zipcode,
		ExpiredAt: order.ExpireAt,
	}

	for _, item := range order.OrderItems {
		m.Items = append(m.Items, db.OrderItemModel{
			ProductID:        item.ProductID,
			Quantity:         item.Quantity,
			SnapshotPrice:    item.SnapshotPrice,
			SnapshotCurrency: item.SnapshotCurrency,
			Price:            item.Price,
		})
	}

	return m
}

func toDomainOrder(model *db.OrderModel) *domain.Order {
	order := &domain.Order{
		ID:        model.ID,
		UserID:    model.UserID,
		Phone:     model.Phone,
		Status:    domain.OrderStatus(model.Status),
		CreatedAt: model.CreatedAt,
		ExpireAt:  model.ExpiredAt,
		Amt: domain.Amount{
			Currency: model.Currency,
			Total:    model.Total,
		},
		Addr: domain.Address{
			Street:  model.Street,
			City:    model.City,
			State:   model.State,
			Country: model.Country,
			Zipcode: model.ZipCode,
		},
	}

	for _, itemModel := range model.Items {
		order.OrderItems = append(order.OrderItems, domain.OrderItem{
			ProductID:        itemModel.ProductID,
			Quantity:         itemModel.Quantity,
			SnapshotPrice:    itemModel.SnapshotPrice,
			SnapshotCurrency: itemModel.SnapshotCurrency,
			Price:            itemModel.Price,
		})
	}

	return order
}
