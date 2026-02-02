//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/order/job"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type OrderIntegrationSuite struct {
	suite.Suite
	mysqlContainer testcontainers.Container
	redisContainer testcontainers.Container
	kafkaContainer testcontainers.Container
	db             *gorm.DB
	redis          redis.Cmdable
	kafkaProducer  sarama.SyncProducer
	kafkaBroker    string

	// 组件
	orderRepo  domain.OrderRepository
	outboxRepo domain.OutboxRepository
	txManager  domain.TxManager
	producer   mq.SaramaProducer
	orderCache cache.OrderCache

	// UseCase
	createOrderUC       *usecase.CreateOrderUseCase
	listUserOrderUC     *usecase.ListUserOrderUseCase
	changeOrderStatusUC *usecase.ChangeOrderStatusUseCase
	batchCancelOrderUC  *usecase.BatchCancelOrderUseCase

	// Job
	checkExpiredJob *job.CheckExpiredJob
	outboxWorkerJob *job.OutboxWorkerJob
}

func TestOrderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}
	suite.Run(t, new(OrderIntegrationSuite))
}

func (s *OrderIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 启动 MySQL 容器
	mysqlReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "order_test",
		},
		WaitingFor: wait.ForLog("ready for connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	mysqlContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: mysqlReq,
		Started:          true,
	})
	require.NoError(s.T(), err)
	s.mysqlContainer = mysqlContainer

	mysqlHost, err := mysqlContainer.Host(ctx)
	require.NoError(s.T(), err)
	mysqlPort, err := mysqlContainer.MappedPort(ctx, "3306")
	require.NoError(s.T(), err)

	dsn := "root:root@tcp(" + mysqlHost + ":" + mysqlPort.Port() + ")/order_test?charset=utf8mb4&parseTime=True&loc=Local"

	// 重试连接MySQL
	var gormDB *gorm.DB
	for i := 0; i < 5; i++ {
		gormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, err := gormDB.DB()
			if err == nil && sqlDB.Ping() == nil {
				break
			}
		}
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(s.T(), err)
	s.db = gormDB

	// 自动迁移表结构
	err = db.InitTables(gormDB)
	require.NoError(s.T(), err)

	// 启动 Redis 容器
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	require.NoError(s.T(), err)
	s.redisContainer = redisContainer

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(s.T(), err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(s.T(), err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort.Port(),
	})
	s.redis = redisClient

	// 启动 Kafka 容器
	kafkaReq := testcontainers.ContainerRequest{
		Image:        "apache/kafka:latest",
		ExposedPorts: []string{"9092/tcp"},
		WaitingFor:   wait.ForLog("Kafka Server started"),
	}
	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: kafkaReq,
		Started:          true,
	})
	require.NoError(s.T(), err)
	s.kafkaContainer = kafkaContainer

	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoError(s.T(), err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9092")
	require.NoError(s.T(), err)
	s.kafkaBroker = kafkaHost + ":" + kafkaPort.Port()

	// 创建 Kafka Producer
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	kafkaProducer, err := sarama.NewSyncProducer([]string{s.kafkaBroker}, config)
	require.NoError(s.T(), err)
	s.kafkaProducer = kafkaProducer

	// 初始化组件
	testLogger := logger.NewNopLogger()
	s.orderCache = cache.NewRedisOrderCache(redisClient)
	s.orderRepo = repository.NewOrderRepository(gormDB, s.orderCache, testLogger)
	s.outboxRepo = repository.NewOutboxRepository(gormDB)
	s.txManager = db.NewGormTxManager(gormDB)
	s.producer = mq.NewSaramaProducer(kafkaProducer)

	// 初始化 UseCase
	s.createOrderUC = usecase.NewCreateOrderUseCase(s.orderRepo, testLogger)
	s.listUserOrderUC = usecase.NewListUserOrderUseCase(s.orderRepo, testLogger)
	s.changeOrderStatusUC = usecase.NewChangeOrderStatusUseCase(
		s.orderRepo,
		s.outboxRepo,
		s.producer,
		s.txManager,
		testLogger,
	)
	s.batchCancelOrderUC = usecase.NewBatchCancelOrderUseCase(
		s.orderRepo,
		s.outboxRepo,
		s.producer,
		s.txManager,
		testLogger,
	)

	// 初始化 Job
	s.checkExpiredJob = job.NewCheckExpiredJob(s.orderRepo, s.batchCancelOrderUC, testLogger)
	s.outboxWorkerJob = job.NewOutboxWorkerJob(s.outboxRepo, s.producer, testLogger)
}

func (s *OrderIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(ctx)
	}
	if s.redisContainer != nil {
		s.redisContainer.Terminate(ctx)
	}
	if s.kafkaContainer != nil {
		s.kafkaContainer.Terminate(ctx)
	}
	if s.kafkaProducer != nil {
		s.kafkaProducer.Close()
	}
}

func (s *OrderIntegrationSuite) SetupTest() {
	ctx := context.Background()
	// 清空数据库 - 使用DELETE避免外键约束问题
	s.db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	s.db.Exec("DELETE FROM order_items")
	s.db.Exec("DELETE FROM orders")
	s.db.Exec("DELETE FROM outbox_events")
	// 重置auto_increment让每个测试ID从1开始
	s.db.Exec("ALTER TABLE orders AUTO_INCREMENT = 1")
	s.db.Exec("ALTER TABLE order_items AUTO_INCREMENT = 1")
	s.db.Exec("ALTER TABLE outbox_events AUTO_INCREMENT = 1")
	s.db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	// 清空 Redis
	s.redis.FlushDB(ctx)
}

// ==================== Repository 测试 ====================

func (s *OrderIntegrationSuite) TestCreateOrder() {
	ctx := context.Background()

	order := &domain.Order{
		UserID: 1001,
		Phone:  "13800138000",
		Status: domain.OrderStatusCreated,
		Amt: domain.Amount{
			Currency: "CNY",
			Total:    9900,
		},
		Addr: domain.Address{
			Street:  "测试街道",
			City:    "北京",
			State:   "北京市",
			Country: "中国",
			Zipcode: "100000",
		},
		OrderItems: []domain.OrderItem{
			{
				ProductID:        2001,
				Quantity:         2,
				SnapshotPrice:    4950,
				SnapshotCurrency: "CNY",
				Price:            4950,
			},
		},
		ExpireAt: time.Now().Add(30 * time.Minute),
	}

	err := s.orderRepo.Save(ctx, order)
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), order.ID)

	// 验证数据库
	var dbOrder db.OrderModel
	err = s.db.WithContext(ctx).Preload("Items").Where("id = ?", order.ID).First(&dbOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1001), dbOrder.UserID)
	assert.Equal(s.T(), uint8(domain.OrderStatusCreated), dbOrder.Status)

	// 调试：直接查询order_items表
	var itemCount int64
	s.db.Model(&db.OrderItemModel{}).Where("order_id = ?", order.ID).Count(&itemCount)
	s.T().Logf("Order ID=%d, Items in DB: %d, Preloaded Items: %d", order.ID, itemCount, len(dbOrder.Items))

	assert.Greater(s.T(), len(dbOrder.Items), 0)

	// 验证缓存（应该写入了）
	time.Sleep(100 * time.Millisecond)
	key := fmt.Sprintf("order:%d", order.ID)
	exists, err := s.redis.Exists(ctx, key).Result()
	require.NoError(s.T(), err)
	assert.Greater(s.T(), exists, int64(0))
}

func (s *OrderIntegrationSuite) TestListByUserID_FirstPage_Cache() {
	ctx := context.Background()

	// 创建3个订单
	for i := 0; i < 3; i++ {
		order := &domain.Order{
			UserID:   1001,
			Phone:    "13800138000",
			Status:   domain.OrderStatusCreated,
			Amt:      domain.Amount{Currency: "CNY", Total: 9900},
			Addr:     domain.Address{City: "北京"},
			ExpireAt: time.Now().Add(30 * time.Minute),
		}
		err := s.orderRepo.Save(ctx, order)
		require.NoError(s.T(), err)
		time.Sleep(10 * time.Millisecond) // 确保创建时间不同
	}

	// 第一次查询（走DB，回填缓存）
	orders1, nextCursor1, err := s.orderRepo.ListByUserID(ctx, 1001, 0, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), orders1, 3)
	assert.Equal(s.T(), int64(0), nextCursor1) // 没有下一页

	// 第二次查询（走缓存）
	orders2, nextCursor2, err := s.orderRepo.ListByUserID(ctx, 1001, 0, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), orders2, 3)
	assert.Equal(s.T(), int64(0), nextCursor2)

	// 验证结果一致
	assert.Equal(s.T(), orders1[0].ID, orders2[0].ID)
}

func (s *OrderIntegrationSuite) TestUpdateStatus_DeleteCache() {
	ctx := context.Background()

	// 创建订单
	order := &domain.Order{
		UserID:   1001,
		Status:   domain.OrderStatusCreated,
		Amt:      domain.Amount{Currency: "CNY", Total: 9900},
		Addr:     domain.Address{City: "北京"},
		ExpireAt: time.Now().Add(30 * time.Minute),
	}
	err := s.orderRepo.Save(ctx, order)
	require.NoError(s.T(), err)

	// 更新状态
	order.Status = domain.OrderStatusPaid
	err = s.orderRepo.UpdateStatus(ctx, order)
	require.NoError(s.T(), err)

	// 验证数据库
	var dbOrder db.OrderModel
	err = s.db.WithContext(ctx).Where("id = ?", order.ID).First(&dbOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(domain.OrderStatusPaid), dbOrder.Status)
}

func (s *OrderIntegrationSuite) TestFindExpiredOrders() {
	ctx := context.Background()

	// 创建过期订单
	expiredOrder := &domain.Order{
		UserID:   1001,
		Status:   domain.OrderStatusCreated,
		Amt:      domain.Amount{Currency: "CNY", Total: 9900},
		Addr:     domain.Address{City: "北京"},
		ExpireAt: time.Now().Add(-1 * time.Minute), // 已过期
	}
	err := s.orderRepo.Save(ctx, expiredOrder)
	require.NoError(s.T(), err)

	// 创建未过期订单
	validOrder := &domain.Order{
		UserID:   1002,
		Status:   domain.OrderStatusCreated,
		Amt:      domain.Amount{Currency: "CNY", Total: 8800},
		Addr:     domain.Address{City: "上海"},
		ExpireAt: time.Now().Add(30 * time.Minute),
	}
	err = s.orderRepo.Save(ctx, validOrder)
	require.NoError(s.T(), err)

	// 查找过期订单
	orders, err := s.orderRepo.FindExpiredOrders(ctx, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), orders, 1)
	assert.Equal(s.T(), expiredOrder.ID, orders[0].ID)
}

// ==================== UseCase 测试 ====================

func (s *OrderIntegrationSuite) TestChangeOrderStatus_WithOutbox() {
	ctx := context.Background()

	// 创建订单
	order := &domain.Order{
		UserID:   1001,
		Status:   domain.OrderStatusCreated,
		Amt:      domain.Amount{Currency: "CNY", Total: 9900},
		Addr:     domain.Address{City: "北京"},
		ExpireAt: time.Now().Add(30 * time.Minute),
	}
	err := s.orderRepo.Save(ctx, order)
	require.NoError(s.T(), err)

	// 修改状态（应该写outbox）
	cmd := usecase.ChangeOrderStatusCmd{
		OrderID:     order.ID,
		OrderStatus: domain.OrderStatusPaid,
	}
	err = s.changeOrderStatusUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)

	// 验证数据库状态已更新
	var dbOrder db.OrderModel
	err = s.db.WithContext(ctx).Where("id = ?", order.ID).First(&dbOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(domain.OrderStatusPaid), dbOrder.Status)

	// 验证outbox已写入
	time.Sleep(100 * time.Millisecond)
	var outboxEvent db.OutboxEventModel
	err = s.db.WithContext(ctx).Where("event_type = ?", "order.status.changed").First(&outboxEvent).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(db.EventStatusPending), uint8(outboxEvent.Status))

	var payload domain.OrderStatusUpdateEvent
	err = json.Unmarshal([]byte(outboxEvent.Payload), &payload)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), order.ID, payload.OrderID)
	assert.Equal(s.T(), domain.OrderStatusPaid, payload.Status)
}

func (s *OrderIntegrationSuite) TestBatchCancelOrder_WithTransaction() {
	ctx := context.Background()

	// 创建多个订单
	orders := make([]*domain.Order, 3)
	for i := 0; i < 3; i++ {
		order := &domain.Order{
			UserID:   1001,
			Status:   domain.OrderStatusCreated,
			Amt:      domain.Amount{Currency: "CNY", Total: 9900},
			Addr:     domain.Address{City: "北京"},
			ExpireAt: time.Now().Add(-1 * time.Minute), // 已过期
		}
		err := s.orderRepo.Save(ctx, order)
		require.NoError(s.T(), err)
		orders[i] = order
	}

	// 批量取消
	err := s.batchCancelOrderUC.Execute(ctx, orders)
	require.NoError(s.T(), err)

	// 验证所有订单状态已更新
	for _, order := range orders {
		var dbOrder db.OrderModel
		err = s.db.WithContext(ctx).Where("id = ?", order.ID).First(&dbOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), uint8(domain.OrderStatusCanceled), dbOrder.Status)
	}

	// 验证outbox已批量写入
	var count int64
	s.db.WithContext(ctx).Model(&db.OutboxEventModel{}).
		Where("event_type = ?", "order.status.changed").
		Count(&count)
	assert.Equal(s.T(), int64(3), count)
}

// ==================== Job 测试 ====================

func (s *OrderIntegrationSuite) TestCheckExpiredJob_CancelExpiredOrders() {
	ctx := context.Background()

	// 创建过期订单
	for i := 0; i < 2; i++ {
		order := &domain.Order{
			UserID:   1001,
			Status:   domain.OrderStatusCreated,
			Amt:      domain.Amount{Currency: "CNY", Total: 9900},
			Addr:     domain.Address{City: "北京"},
			ExpireAt: time.Now().Add(-1 * time.Minute),
		}
		err := s.orderRepo.Save(ctx, order)
		require.NoError(s.T(), err)
	}

	// 创建未过期订单
	validOrder := &domain.Order{
		UserID:   1002,
		Status:   domain.OrderStatusCreated,
		Amt:      domain.Amount{Currency: "CNY", Total: 8800},
		Addr:     domain.Address{City: "上海"},
		ExpireAt: time.Now().Add(30 * time.Minute),
	}
	err := s.orderRepo.Save(ctx, validOrder)
	require.NoError(s.T(), err)

	// 执行Job
	err = s.checkExpiredJob.Run()
	require.NoError(s.T(), err)

	// 验证过期订单已取消
	var canceledCount int64
	s.db.WithContext(ctx).Model(&db.OrderModel{}).
		Where("status = ?", domain.OrderStatusCanceled).
		Count(&canceledCount)
	assert.Equal(s.T(), int64(2), canceledCount)

	// 验证未过期订单仍为Created
	var dbValidOrder db.OrderModel
	err = s.db.WithContext(ctx).Where("id = ?", validOrder.ID).First(&dbValidOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(domain.OrderStatusCreated), dbValidOrder.Status)
}

func (s *OrderIntegrationSuite) TestOutboxWorkerJob_SendPendingEvents() {
	ctx := context.Background()

	// 手动创建pending outbox事件
	event := domain.OrderStatusUpdateEvent{
		OrderID: 123,
		Status:  domain.OrderStatusPaid,
	}
	err := s.outboxRepo.Add(ctx, "order.status.changed", event)
	require.NoError(s.T(), err)

	// 执行Job
	err = s.outboxWorkerJob.Run()
	require.NoError(s.T(), err)

	// 验证事件状态已更新为Sent
	time.Sleep(200 * time.Millisecond)
	var outboxEvent db.OutboxEventModel
	err = s.db.WithContext(ctx).Where("event_type = ?", "order.status.changed").First(&outboxEvent).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(db.EventStatusSent), uint8(outboxEvent.Status))
}

// ==================== Keyset分页测试 ====================

func (s *OrderIntegrationSuite) TestKeysetPagination() {
	ctx := context.Background()

	// 创建20个订单
	for i := 0; i < 20; i++ {
		order := &domain.Order{
			UserID:   1001,
			Status:   domain.OrderStatusCreated,
			Amt:      domain.Amount{Currency: "CNY", Total: 9900},
			Addr:     domain.Address{City: "北京"},
			ExpireAt: time.Now().Add(30 * time.Minute),
		}
		err := s.orderRepo.Save(ctx, order)
		require.NoError(s.T(), err)
		time.Sleep(10 * time.Millisecond)
	}

	// 第一页
	page1, cursor1, err := s.orderRepo.ListByUserID(ctx, 1001, 0, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), page1, 10)
	assert.NotZero(s.T(), cursor1) // 有下一页

	// 第二页
	page2, cursor2, err := s.orderRepo.ListByUserID(ctx, 1001, cursor1, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), page2, 10)
	assert.Equal(s.T(), int64(0), cursor2) // 没有更多数据

	// 验证无重复
	ids := make(map[int64]bool)
	for _, o := range page1 {
		ids[o.ID] = true
	}
	for _, o := range page2 {
		assert.False(s.T(), ids[o.ID], "发现重复ID: %d", o.ID)
	}
}
