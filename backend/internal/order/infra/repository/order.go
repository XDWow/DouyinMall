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

// ErrRecordNotFound 宸茬Щ鍒癲omain灞?
// var ErrRecordNotFound = gorm.ErrRecordNotFound

const (
	orderTTL         = 30 * time.Second
	userOrderListTTL = 30 * time.Second
	pageLimit        = 10
)

// 缂撳瓨鍦烘櫙鍒嗘瀽锛?
// 涓€鑷存€ч珮 + 浣庡苟鍙?鈫?鐩存帴 DB
// 涓€鑷存€ч珮 + 楂樺苟鍙?鈫?鍒犵紦瀛?cache-aside锛岀珛鍒绘嬁鍒版渶鏂版暟鎹? + 鐭璗TL锛堝垹闄ょ紦瀛樺け璐ヤ簡鐨勫厹搴曪紝鏈€澶氬欢杩熶竴浼氳嚜鍔ㄨ繃鏈燂級锛堟牳蹇冿細涓嶄俊浠诲湴浣跨敤缂撳瓨锛?
// 涓€鑷存€т綆 + 楂樺苟鍙?鈫?缁存姢缂撳瓨
// 涓€鑷存€т綆 + 浣庡苟鍙?鈫?鐢氳嚦涓嶉渶瑕?Redis

// 璁㈠崟绯荤粺閲囩敤鏃佽矾缂撳瓨锛圕ache Aside锛?
// DB 涓哄敮涓€浜嬪疄婧愶紝缂撳瓨浠呯敤浜庡姞閫熼珮骞跺彂璇?
// 閫氳繃 TTL 鍏滃簳锛屽厑璁稿嚑鍗佺鐨勫睍绀烘粸鍚庯紝浣嗕繚璇佹渶缁堜竴鑷存€?
// 涓嶅厑璁哥紦瀛樺弬涓庝笟鍔＄姸鎬佹紨杩?
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
	// GORM浼氳嚜鍔ㄨ缃甇rderItems鐨凮rderID澶栭敭
	if err := conn.Create(orderModel).Error; err != nil {
		return err
	}
	// 鍥炲啓鐢熸垚鐨処D鍒癲omain瀵硅薄
	order.ID = orderModel.ID

	// 鍐欑紦瀛橈紝澶辫触浜嗘墦鏃ュ織灏辫锛屽ぇ涓嶄簡涓嶅姞閫熶簡鑰屽凡
	data, err := json.Marshal(orderModel)
	if err != nil {
		repo.log.Warn("璁㈠崟搴忓垪鍖栧け璐ワ紝鏃犳硶鍐欏叆缂撳瓨", logger.Error(err), logger.Int64("orderID", order.ID))
		return nil
	}
	if err := repo.cache.Set(ctx, orderKey(orderModel.ID), string(data), orderTTL); err != nil {
		repo.log.Warn("鍐欏叆璁㈠崟缂撳瓨澶辫触", logger.Error(err), logger.Int64("orderID", order.ID))
	}

	members := map[string]float64{
		strconv.FormatInt(orderModel.ID, 10): float64(orderModel.CreatedAt.UnixMilli()),
	}
	// 浣跨敤ZAddWithLimit淇濇寔鍥哄畾澶у皬
	if err := repo.cache.ZAddWithLimit(ctx, userOrderListKey(orderModel.UserID), members, pageLimit, userOrderListTTL); err != nil {
		repo.log.Warn("鍐欏叆鐢ㄦ埛璁㈠崟鍒楄〃缂撳瓨澶辫触", logger.Error(err), logger.Int64("userID", orderModel.UserID))
	}

	return nil
}

func (repo *orderRepository) FindByID(ctx context.Context, orderID int64) (domain.Order, error) {
	conn := db.DBFromContext(ctx, repo.db)
	var orderModel db.OrderModel
	err := conn.
		Preload("Items"). // 棰勫姞杞借鍗曢」
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
		conn = conn.Clauses(clause.Locking{Strength: "UPDATE"}) // sql 璇彞浼氬甫涓?for update
	}
	var models []db.OrderModel
	if err := conn.
		Preload("Items"). // 鎶?Items 杩欎釜鍏宠仈瀛楁椤烘墜涓€璧锋煡鍑烘潵锛屼笉鐒堕粯璁ゅ彧鏌ヤ富琛?
		Where("id IN ?", orderIDs).
		Find(&models).Error; err != nil {
		return nil, err
	}

	// 鏁版嵁搴撴煡璇㈡槸鏃犲簭鐨勶紝閫氳繃 map 鏉ヤ娇杩斿洖鐨?order 璺熶紶杩涙潵鐨?orderID 椤哄簭瀵瑰簲
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
	// 杩欓噷鍒犻櫎缂撳瓨澶辫触浼氬鑷存暟鎹笉涓€鑷达紝鍙兘渚濊禆鐭璗TL鍏滃簳锛屾棤鑳界殑涓堝か
	err := repo.cache.Del(ctx, orderKey(orderID))
	if err != nil {
		repo.log.Warn("鍒犻櫎璁㈠崟缂撳瓨澶辫触", logger.Error(err), logger.Int64("orderID", order.ID))
	}
	return nil
}

// 鐜板湪鐨勫満鏅槸锛氭壒閲忓彇娑堣鍗?
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
	// 鍙紦瀛樼涓€椤碉紙cursor=0鏃剁殑鐑偣鏁版嵁锛?
	if cursor == 0 && limit <= pageLimit {
		// 1. 浠嶼Set鑾峰彇orderID鍒楄〃
		idStrs, err := repo.cache.ZRange(ctx, userOrderListKey(userID), 0, int64(limit-1), true)
		if err != nil {
			repo.log.Warn("ZRange 鏌ヨ澶辫触锛屽洖閫€鍒版暟鎹簱", logger.Error(err), logger.Int64("userID", userID))
		} else if len(idStrs) > 0 {
			orderIDs := make([]int64, 0, len(idStrs))
			for _, idStr := range idStrs {
				if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
					orderIDs = append(orderIDs, id)
				}
			}
			if len(orderIDs) > 0 {
				// 2. 鎵归噺MGet鏌ヨ缂撳瓨
				keys := make([]string, len(orderIDs))
				for i, id := range orderIDs {
					keys[i] = orderKey(id)
				}

				dataList, err := repo.cache.MGet(ctx, keys...)
				if err != nil {
					repo.log.Warn("MGet 鏌ヨ澶辫触锛屽洖閫€鍒版暟鎹簱", logger.Error(err), logger.Int64("userID", userID))
				} else {
					// 3. 鍒嗙鍛戒腑鍜屾湭鍛戒腑鐨刼rderIDs
					orders := make([]*domain.Order, 0, len(orderIDs))
					missIDs := make([]int64, 0)
					missIndexes := make(map[int64]int) // orderID -> orders涓殑浣嶇疆

					for i, data := range dataList {
						if data == nil {
							// 缂撳瓨鏈懡涓?
							missIDs = append(missIDs, orderIDs[i])
							missIndexes[orderIDs[i]] = len(orders)
							orders = append(orders, nil) // 鍗犱綅
						} else {
							var order domain.Order
							if err := json.Unmarshal([]byte(*data), &order); err != nil {
								repo.log.Warn("鍙嶅簭鍒楀寲璁㈠崟澶辫触", logger.Error(err), logger.Int64("orderID", orderIDs[i]))
								// 鍙嶅簭鍒楀寲澶辫触锛屽綋浣渕iss澶勭悊
								missIDs = append(missIDs, orderIDs[i])
								missIndexes[orderIDs[i]] = len(orders)
								orders = append(orders, nil)
							} else {
								orders = append(orders, &order)
							}
						}
					}

					// 4. 鍘籇B琛ュ厖鏌ヨ鏈懡涓殑璁㈠崟
					if len(missIDs) > 0 {
						var missModels []db.OrderModel
						err := db.DBFromContext(ctx, repo.db).
							Preload("Items").
							Where("id IN ?", missIDs).
							Find(&missModels).Error
						if err != nil {
							repo.log.Error("鏁版嵁搴撴煡璇㈢己澶辫鍗曞け璐?, logger.Error(err))
							return nil, 0, err
						}

						// 5. 鍥炲～鍒扮粨鏋?orders锛屽苟鍐欏洖缂撳瓨

						for _, model := range missModels {
							order := toDomainOrder(&model)

							if idx, ok := missIndexes[model.ID]; ok {
								orders[idx] = order
							}

							// 鍐欏洖缂撳瓨
							if data, e := json.Marshal(&order); e == nil {
								repo.cache.Set(ctx, orderKey(order.ID), string(data), orderTTL)
							}
						}
					}

					// 6. 杩囨护鎺変粛鐒朵负绌虹殑鍗犱綅锛圖B涓篃涓嶅瓨鍦紝璇存槑缂撳瓨list鏈夎剰鏁版嵁锛?
					validOrders := make([]*domain.Order, 0, len(orders))
					hasInvalidData := false
					for _, o := range orders {
						if o != nil && o.ID != 0 {
							validOrders = append(validOrders, o)
						} else {
							hasInvalidData = true
						}
					}

					// 鏁版嵁搴撴病鏈夛紙鐪熺浉锛夛紝浣嗘槸list绔熺劧鏈夛紝閭ｅ氨鏄剰鏁版嵁锛屽垹闄ist缂撳瓨
					if hasInvalidData {
						repo.log.Warn("缂撳瓨list瀛樺湪鑴忔暟鎹紝鍒犻櫎list缂撳瓨", logger.Int64("userID", userID))
						repo.cache.Del(ctx, userOrderListKey(userID))
					}

					// 璁＄畻nextCursor
					if len(validOrders) < limit { // 娌℃湁涓嬩竴椤典簡
						return validOrders, 0, nil
					}
					nextCursor = validOrders[len(validOrders)-1].ID
					return validOrders, nextCursor, nil
				}
			}
		}
	}

	// 闈為椤碉紝涓嶈蛋缂撳瓨锛岀洿鎺B鏌ヨ
	var models []db.OrderModel
	// Limit(limit+1)鏉ュ垽鏂槸鍚﹁繕鏈変笅涓€椤?
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

	// 濡傛灉鏌ヨ缁撴灉瓒呰繃limit锛岃鏄庢湁涓嬩竴椤?
	hasMore := len(models) > limit
	if hasMore {
		models = models[:limit] // 鍙繑鍥瀕imit鏉?
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

	// limit > 0 鎵嶉檺鍒舵暟閲忥紝鍚﹀垯鏌ヨ鎵€鏈?
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

// 鍒楀嚭鏌愮敤鎴锋煇鐘舵€佺殑璁㈠崟鍒楄〃锛岃繖涓笉鏄儹鐐规暟鎹惂锛屼笉缂撳瓨浜?
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


