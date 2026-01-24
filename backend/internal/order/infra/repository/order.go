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
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

var ErrRecordNotFound = gorm.ErrRecordNotFound

const (
	orderTTL         = 30 * time.Second
	userOrderListTTL = 30 * time.Second
)

// 在订单这种强一致、高频读写场景下，
// 无法通过精确维护缓存来保证数据正确性，不推荐精确地维护user订单列表缓存
// 缓存只能作为加速层，一旦出现不确定性就必须回退数据库
// 因此最稳妥的方案是：粗暴失效 + 短 TTL
// 用数据库保证正确性，用缓存换取性能

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
	if err := repo.db.WithContext(ctx).Create(orderModel).Error; err != nil {
		return err
	}
	// 写缓存，失败了打日志就行，大不了不加速了而已
	data, err := json.Marshal(order)
	if err != nil {
		repo.log.Warn("订单序列化失败，无法写入缓存", logger.Error(err), logger.Int64("orderID", order.ID))
		return nil
	}
	if err := repo.cache.Set(ctx, orderKey(order.ID), string(data), orderTTL); err != nil {
		repo.log.Warn("写入订单缓存失败", logger.Error(err), logger.Int64("orderID", order.ID))
	}

	members := map[string]float64{
		strconv.FormatInt(order.ID, 10): float64(order.CreatedAt.UnixMilli()),
	}
	if err := repo.cache.ZAdd(ctx, userOrderListKey(order.UserID), members, userOrderListTTL); err != nil {
		repo.log.Warn("写入用户订单列表缓存失败", logger.Error(err), logger.Int64("userID", order.UserID))
	}

	return nil
}

func (repo *orderRepository) UpdateStatus(ctx context.Context, order *domain.Order) error {
	res := repo.db.WithContext(ctx).
		Model(&db.OrderModel{}).
		Where("id = ? && status = ?", order.ID, 1).
		Update("status", order.Status.AsUint8())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// 这里删除缓存失败会导致数据不一致，只能依赖短TTL兜底，无能的丈夫
	err := repo.cache.Del(ctx, orderKey(order.ID))
	if err != nil {
		repo.log.Warn("删除订单缓存失败", logger.Error(err), logger.Int64("orderID", order.ID))
	}
	return nil
}

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
		return gorm.ErrRecordNotFound
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
	offset,
	limit int,
) ([]domain.Order, error) {
	// 只缓存第一页（热点数据），这里有个重要的点：拒绝拼好饭
	// Redis 只是加速器，DB 是最终事实源，如果redis数据不完整或异常，说明它和数据库不一致
	// 对于订单业务来说，数据正确性优先级高于性能
	if offset == 0 {
		idStrs, err := repo.cache.ZRange(ctx, userOrderListKey(userID), 0, int64(limit-1), true)
		if err == nil && len(idStrs) > 0 {
			orderIDs := make([]int64, 0, len(idStrs))
			for _, idStr := range idStrs {
				if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
					orderIDs = append(orderIDs, id)
				}
			}

			if len(orderIDs) > 0 {
				orders := make([]domain.Order, 0, len(orderIDs))
				for _, id := range orderIDs {
					data, err := repo.cache.Get(ctx, orderKey(id))
					if err != nil || err == redis.Nil {
						break // 任何一个miss，立即中断，拒绝拼好饭
					}
					var order domain.Order
					if err = json.Unmarshal([]byte(data), &order); err != nil {
						break // 反序列化失败，数据有问题，中断
					}
					orders = append(orders, order)
				}

				// 只有全部命中才返回缓存，否则fallback到DB
				if len(orders) == len(orderIDs) {
					return orders, nil
				}
			}
		}
	}

	// 缓存miss或非首页，查DB
	var models []db.OrderModel
	err := repo.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	orders := make([]domain.Order, len(models))
	for i, model := range models {
		orders[i] = domain.Order{
			ID:        model.ID,
			UserID:    model.UserID,
			Phone:     model.Phone,
			Status:    domain.OrderStatus(model.Status),
			CreatedAt: model.CreatedAt,
			ExpireAt:  model.ExpiredAt,
		}
	}

	// 首页查询成功，回写缓存
	if offset == 0 && len(orders) > 0 {
		members := make(map[string]float64, len(orders))
		for i := range orders {
			members[strconv.FormatInt(orders[i].ID, 10)] = float64(orders[i].CreatedAt.Unix())
			if data, err := json.Marshal(&orders[i]); err == nil {
				repo.cache.Set(ctx, orderKey(orders[i].ID), string(data), orderTTL)
			}
		}
		repo.cache.ZAdd(ctx, userOrderListKey(userID), members, userOrderListTTL)
	}

	return orders, nil
}

func (repo *orderRepository) FindExpiredOrders(ctx context.Context, limit int) ([]*domain.Order, error) {
	var models []db.OrderModel
	query := repo.db.WithContext(ctx).
		Where("status = ? AND expired_at < ?", domain.OrderStatusPending, time.Now())

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
		orders[i] = &domain.Order{
			ID:        model.ID,
			UserID:    model.UserID,
			Phone:     model.Phone,
			Status:    domain.OrderStatus(model.Status),
			CreatedAt: model.CreatedAt,
			ExpireAt:  model.ExpiredAt,
		}
	}
	return orders, nil
}

func (repo *orderRepository) ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*domain.Order, error) {
	var orders []*domain.Order
	err := repo.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, status).
		Find(&orders).Error
	return orders, err
}

func toOrderModel(order *domain.Order) *db.OrderModel {
	m := &db.OrderModel{
		ID:       order.ID,
		UserID:   order.UserID,
		Phone:    order.Phone,
		Status:   uint8(order.Status),
		Currency: order.Amt.Currency,
		Total:    order.Amt.Total,
		Street:   order.Addr.Street,
		City:     order.Addr.City,
		State:    order.Addr.State,
		Country:  order.Addr.Country,
		ZipCode:  order.Addr.Zipcode,
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
