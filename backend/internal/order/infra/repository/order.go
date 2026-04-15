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
	"gorm.io/gorm/clause"
)

// ErrRecordNotFound 已定义在 domain 包。

const (
	orderTTL         = 30 * time.Second
	userOrderListTTL = 30 * time.Second
	pageLimit        = 10
)

// 缓存场景（一致性 / 并发）粗分：
// 一致性高 + 低并发 → 直接查 DB
// 一致性高 + 高并发 → 删缓存（cache-aside，立即读最新）+ 短 TTL（删缓存失败时的兜底，最多延迟一会儿自动过期）；核心：不盲信缓存
// 一致性低 + 高并发 → 维护缓存
// 一致性低 + 低并发 → 甚至可以不用 Redis

// 订单采用旁路缓存（Cache Aside）。
// DB 为唯一事实源，缓存只用于加速高并发读。
// 通过 TTL 兜底，允许几十毫秒级展示滞后，但最终与 DB 一致。
// 不允许缓存参与业务状态演进。
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
	conn := db.DBFromContext(ctx, repo.db)
	orderModel := toOrderModel(order)
	// GORM 会自动设置 OrderItems 的 OrderID 外键
	if err := conn.Create(orderModel).Error; err != nil {
		return err
	}
	// 回写自增 ID 到 domain 对象
	order.ID = orderModel.ID

	// 写缓存；失败只打日志，大不了少一层加速
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
	// 使用 ZAddWithLimit 保持列表固定长度
	if err := repo.cache.ZAddWithLimit(ctx, userOrderListKey(orderModel.UserID), members, pageLimit, userOrderListTTL); err != nil {
		repo.log.Warn("写入用户订单列表缓存失败", logger.Error(err), logger.Int64("userID", orderModel.UserID))
	}

	return nil
}

func (repo *orderRepository) FindByID(ctx context.Context, orderID int64) (domain.Order, error) {
	conn := db.DBFromContext(ctx, repo.db)
	var orderModel db.OrderModel
	err := conn.
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

func (repo *orderRepository) FindByIDs(ctx context.Context, orderIDs []int64) ([]*domain.Order, error) {
	return repo.findByIDs(ctx, orderIDs, false)
}

func (repo *orderRepository) FindByIDsForUpdate(ctx context.Context, orderIDs []int64) ([]*domain.Order, error) {
	return repo.findByIDs(ctx, orderIDs, true)
}

func (repo *orderRepository) findByIDs(ctx context.Context, orderIDs []int64, forUpdate bool) ([]*domain.Order, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}

	conn := db.DBFromContext(ctx, repo.db)
	if forUpdate {
		conn = conn.Clauses(clause.Locking{Strength: "UPDATE"}) // SQL 会带 FOR UPDATE
	}
	var models []db.OrderModel
	if err := conn.
		Preload("Items"). // 把 Items 一并查出，否则默认只查主表
		Where("id IN ?", orderIDs).
		Find(&models).Error; err != nil {
		return nil, err
	}

	// DB 查询结果无序，用 map 再按入参 orderID 顺序组装
	ordersByID := make(map[int64]*domain.Order, len(models))
	for _, model := range models {
		order := toDomainOrder(&model)
		ordersByID[order.ID] = order
	}

	orders := make([]*domain.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order, ok := ordersByID[orderID]
		if !ok {
			return nil, domain.ErrRecordNotFound
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (repo *orderRepository) UpdateStatus(ctx context.Context, orderID int64, fromStatus, toStatus domain.OrderStatus) error {
	conn := db.DBFromContext(ctx, repo.db)
	order := domain.Order{ID: orderID}
	fromStatuses := matchStatuses(fromStatus)
	res := conn.
		Model(&db.OrderModel{}).
		Where("id = ? AND status IN ?", orderID, fromStatuses).
		Update("status", toStatus)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	// 此处删缓存失败会导致短暂不一致，只能依赖短 TTL 兜底
	err := repo.cache.Del(ctx, orderKey(orderID))
	if err != nil {
		repo.log.Warn("删除订单缓存失败", logger.Error(err), logger.Int64("orderID", order.ID))
	}
	return nil
}

// 当前用于批量取消订单等场景
func (repo *orderRepository) BatchUpdateStatus(ctx context.Context, orderIDs []int64, fromStatus, toStatus domain.OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}

	conn := db.DBFromContext(ctx, repo.db)
	res := conn.
		Model(&db.OrderModel{}).
		Where("id IN ? AND status = ?", orderIDs, fromStatus).
		Update("status", toStatus)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != int64(len(orderIDs)) {
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
	// 只缓存第一页（cursor=0）的热点数据
	if cursor == 0 && limit <= pageLimit {
		// 1. 从 ZSet 取 orderID 列表
		idStrs, err := repo.cache.ZRange(ctx, userOrderListKey(userID), 0, int64(limit-1), true)
		if err != nil {
			repo.log.Warn("ZRange 查询失败，回退到数据库", logger.Error(err), logger.Int64("userID", userID))
		} else if len(idStrs) > 0 {
			orderIDs := make([]int64, 0, len(idStrs))
			for _, idStr := range idStrs {
				if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
					orderIDs = append(orderIDs, id)
				}
			}
			if len(orderIDs) > 0 {
				// 2. MGet 批量读缓存
				keys := make([]string, len(orderIDs))
				for i, id := range orderIDs {
					keys[i] = orderKey(id)
				}

				dataList, err := repo.cache.MGet(ctx, keys...)
				if err != nil {
					repo.log.Warn("MGet 查询失败，回退到数据库", logger.Error(err), logger.Int64("userID", userID))
				} else {
					// 3. 分离命中与未命中的 orderID
					orders := make([]*domain.Order, 0, len(orderIDs))
					missIDs := make([]int64, 0)
					missIndexes := make(map[int64]int) // orderID -> 在 orders 切片中的下标

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
								// 反序列化失败按未命中处理
								missIDs = append(missIDs, orderIDs[i])
								missIndexes[orderIDs[i]] = len(orders)
								orders = append(orders, nil)
							} else {
								orders = append(orders, &order)
							}
						}
					}

					// 4. 回 DB 补全未命中订单
					if len(missIDs) > 0 {
						var missModels []db.OrderModel
						err := db.DBFromContext(ctx, repo.db).
							Preload("Items").
							Where("id IN ?", missIDs).
							Find(&missModels).Error
						if err != nil {
							repo.log.Error("数据库查询缺失订单失败", logger.Error(err))
							return nil, 0, err
						}

						// 5. 回填到 orders，并写回缓存

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

					// 6. 去掉仍为空的占位（DB 也无则说明 list 缓存脏）
					validOrders := make([]*domain.Order, 0, len(orders))
					hasInvalidData := false
					for _, o := range orders {
						if o != nil && o.ID != 0 {
							validOrders = append(validOrders, o)
						} else {
							hasInvalidData = true
						}
					}

					// DB 无记录但 list 仍有脏数据，删除 list 缓存
					if hasInvalidData {
						repo.log.Warn("缓存 list 存在脏数据，删除 list 缓存", logger.Int64("userID", userID))
						repo.cache.Del(ctx, userOrderListKey(userID))
					}

					// 计算 nextCursor
					if len(validOrders) < limit { // 没有下一页
						return validOrders, 0, nil
					}
					nextCursor = validOrders[len(validOrders)-1].ID
					return validOrders, nextCursor, nil
				}
			}
		}
	}

	// 非首页不走缓存，直接查 DB
	var models []db.OrderModel
	// Limit(limit+1) 用于判断是否还有下一页
	query := db.DBFromContext(ctx, repo.db).
		Preload("Items").
		Where("user_id = ?", userID)
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	err = query.Order("id DESC").
		Limit(limit + 1).
		Find(&models).Error
	if err != nil {
		return nil, cursor, err
	}

	// 若结果超过 limit 说明还有下一页
	hasMore := len(models) > limit
	if hasMore {
		models = models[:limit] // 只返回 limit 条
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
	query := db.DBFromContext(ctx, repo.db).
		Preload("Items").
		Where("status = ? AND expired_at < ?", domain.OrderStatusCreated, time.Now())

	// limit > 0 才限制条数，否则查全部
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

// 按用户与状态列订单；非热点路径，不做缓存
func (repo *orderRepository) ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*domain.Order, error) {
	var models []db.OrderModel
	err := db.DBFromContext(ctx, repo.db).
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
		ID:            order.ID,
		UserID:        order.UserID,
		Phone:         order.Addr.Phone,
		Remark:        order.Remark,
		Status:        uint8(order.Status),
		OrderKind:     order.OrderKind,
		ActivityID:    order.ActivityID,
		Currency:      order.PayableAmount.Currency,
		Total:         order.TotalAmount.Total,
		PayableTotal:  order.PayableAmount.Total,
		DiscountTotal: order.DiscountAmount.Total,
		Street:        order.Addr.Street,
		City:          order.Addr.City,
		State:         order.Addr.State,
		Country:       order.Addr.Country,
		ZipCode:       order.Addr.Zipcode,
		ExpiredAt:     order.ExpireAt,
	}

	for _, item := range order.OrderItems {
		m.Items = append(m.Items, db.OrderItemModel{
			ProductID:        item.ProductID,
			SKUID:            item.SKUID,
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
		ID:         model.ID,
		UserID:     model.UserID,
		Remark:     model.Remark,
		Status:     domain.OrderStatus(model.Status),
		OrderKind:  model.OrderKind,
		ActivityID: model.ActivityID,
		CreatedAt:  model.CreatedAt,
		ExpireAt:   model.ExpiredAt,
		PayableAmount: domain.Amount{
			Currency: model.Currency,
			Total:    model.PayableTotal,
		},
		TotalAmount: domain.Amount{
			Currency: model.Currency,
			Total:    model.Total,
		},
		DiscountAmount: domain.Amount{
			Currency: model.Currency,
			Total:    model.DiscountTotal,
		},
		Addr: domain.Address{
			Street:  model.Street,
			City:    model.City,
			State:   model.State,
			Country: model.Country,
			Zipcode: model.ZipCode,
			Phone:   model.Phone,
		},
	}

	for _, itemModel := range model.Items {
		order.OrderItems = append(order.OrderItems, domain.OrderItem{
			ProductID:        itemModel.ProductID,
			SKUID:            itemModel.SKUID,
			Quantity:         itemModel.Quantity,
			SnapshotPrice:    itemModel.SnapshotPrice,
			SnapshotCurrency: itemModel.SnapshotCurrency,
			Price:            itemModel.Price,
		})
	}

	return order
}

func matchStatuses(status domain.OrderStatus) []domain.OrderStatus {
	return []domain.OrderStatus{status}
}


