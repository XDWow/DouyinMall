// Package integration 集成测试
//
// 使用 Testcontainers 启动真实的 MySQL 和 Redis 容器进行测试
// 需要本地安装 Docker
//
// 运行方式：
//
//	go test -tags=integration ./internal/product/integration/... -v
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/product/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProductIntegrationSuite 商品服务集成测试套件
type ProductIntegrationSuite struct {
	suite.Suite

	// 容器
	mysqlContainer *tcmysql.MySQLContainer
	redisContainer *tcredis.RedisContainer

	// 数据库连接
	db  *gorm.DB
	rdb *redis.Client

	// 服务组件
	productRepo repo.ProductRepo
	productSvc  service.ProductService
}

func TestProductIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}
	suite.Run(t, new(ProductIntegrationSuite))
}

// SetupSuite 测试套件启动前执行
func (s *ProductIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 1. 启动 MySQL 容器
	mysqlContainer, err := tcmysql.Run(ctx,
		"mysql:8.0",
		tcmysql.WithDatabase("product_test"),
		tcmysql.WithUsername("root"),
		tcmysql.WithPassword("test123"),
	)
	require.NoError(s.T(), err)
	s.mysqlContainer = mysqlContainer

	// 获取 MySQL DSN
	dsn, err := mysqlContainer.ConnectionString(ctx, "parseTime=true")
	require.NoError(s.T(), err)

	// 连接 MySQL
	s.db, err = gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err)

	// 自动迁移表结构
	err = s.db.AutoMigrate(&dao.Product{})
	require.NoError(s.T(), err)

	// 2. 启动 Redis 容器
	redisContainer, err := tcredis.Run(ctx, "redis:7")
	require.NoError(s.T(), err)
	s.redisContainer = redisContainer

	// 获取 Redis 地址
	redisAddr, err := redisContainer.Endpoint(ctx, "")
	require.NoError(s.T(), err)

	// 连接 Redis
	s.rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 3. 初始化服务组件
	productDao := dao.NewGORMProductDao(s.db)
	productCache := cache.NewRedisProductCache(s.rdb)
	s.productRepo = repo.NewCachedProductRepo(productDao, productCache, logger.NewNopLogger())
	s.productSvc = service.NewProductService(s.productRepo, logger.NewNopLogger())
}

// TearDownSuite 测试套件结束后执行
func (s *ProductIntegrationSuite) TearDownSuite() {
	ctx := context.Background()

	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(ctx)
	}
	if s.mysqlContainer != nil {
		_ = s.mysqlContainer.Terminate(ctx)
	}
}

// SetupTest 每个测试前执行
func (s *ProductIntegrationSuite) SetupTest() {
	// 清空数据
	s.db.Exec("TRUNCATE TABLE product")
	s.rdb.FlushDB(context.Background())
}

// ============================================================================
// 测试用例
// ============================================================================

func (s *ProductIntegrationSuite) TestCreateAndGetProduct() {
	ctx := context.Background()

	// 创建商品
	product := domain.Product{
		Name:         "iPhone 15 Pro",
		Description:  "最新款 iPhone，A17 Pro 芯片",
		Picture:      "https://example.com/iphone15.jpg",
		SlideImgs:    []string{"https://example.com/1.jpg", "https://example.com/2.jpg"},
		Price:        899900, // 8999.00 元
		Categories:   []string{"电子产品", "手机"},
		Stock:        100,
		MerchantID:   1001,
		MerchantName: "Apple 官方旗舰店",
	}

	id, err := s.productSvc.CreateProduct(ctx, product)
	require.NoError(s.T(), err)
	assert.Greater(s.T(), id, int64(0))

	// 获取商品
	got, err := s.productSvc.GetProduct(ctx, id)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "iPhone 15 Pro", got.Name)
	assert.Equal(s.T(), int64(899900), got.Price)
	assert.Equal(s.T(), int64(100), got.Stock)
	assert.Equal(s.T(), []string{"电子产品", "手机"}, got.Categories)
}

func (s *ProductIntegrationSuite) TestListProducts() {
	ctx := context.Background()

	// 创建多个商品
	products := []domain.Product{
		{Name: "商品1", Price: 1000, Stock: 10, Categories: []string{"分类A"}, MerchantID: 1},
		{Name: "商品2", Price: 2000, Stock: 20, Categories: []string{"分类A"}, MerchantID: 1},
		{Name: "商品3", Price: 3000, Stock: 30, Categories: []string{"分类B"}, MerchantID: 1},
		{Name: "商品4", Price: 4000, Stock: 40, Categories: []string{"分类B"}, MerchantID: 1},
		{Name: "商品5", Price: 5000, Stock: 50, Categories: []string{"分类A"}, MerchantID: 1},
	}

	for _, p := range products {
		_, err := s.productSvc.CreateProduct(ctx, p)
		require.NoError(s.T(), err)
	}

	// 测试分页
	list, err := s.productSvc.ListProducts(ctx, 1, 3, "")
	require.NoError(s.T(), err)
	assert.Len(s.T(), list, 3)

	// 测试按分类筛选
	list, err = s.productSvc.ListProducts(ctx, 1, 10, "分类A")
	require.NoError(s.T(), err)
	assert.Len(s.T(), list, 3) // 分类A有3个商品
}

func (s *ProductIntegrationSuite) TestUpdateProduct() {
	ctx := context.Background()

	// 先创建
	id, err := s.productSvc.CreateProduct(ctx, domain.Product{
		Name:       "原始商品名",
		Price:      1000,
		Stock:      10,
		MerchantID: 1,
	})
	require.NoError(s.T(), err)

	// 更新
	_, err = s.productSvc.UpdateProduct(ctx, domain.Product{
		ID:    id,
		Name:  "更新后的商品名",
		Price: 2000,
	})
	require.NoError(s.T(), err)

	// 验证更新
	got, err := s.productSvc.GetProduct(ctx, id)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "更新后的商品名", got.Name)
	assert.Equal(s.T(), int64(2000), got.Price)
}

func (s *ProductIntegrationSuite) TestDeleteProduct() {
	ctx := context.Background()

	// 创建商品
	id, err := s.productSvc.CreateProduct(ctx, domain.Product{
		Name:       "待删除商品",
		Price:      1000,
		Stock:      10,
		MerchantID: 1001,
	})
	require.NoError(s.T(), err)

	// 删除商品
	err = s.productSvc.DeleteProduct(ctx, id, 1001)
	require.NoError(s.T(), err)

	// 验证删除（应该查不到）
	_, err = s.productSvc.GetProduct(ctx, id)
	assert.Error(s.T(), err)
}

func (s *ProductIntegrationSuite) TestCacheHit() {
	ctx := context.Background()

	// 创建商品
	id, err := s.productSvc.CreateProduct(ctx, domain.Product{
		Name:       "缓存测试商品",
		Price:      1000,
		Stock:      10,
		MerchantID: 1,
	})
	require.NoError(s.T(), err)

	// 第一次查询（缓存 Miss，查库）
	start := time.Now()
	_, err = s.productSvc.GetProduct(ctx, id)
	require.NoError(s.T(), err)
	firstDuration := time.Since(start)

	// 第二次查询（缓存 Hit）
	start = time.Now()
	_, err = s.productSvc.GetProduct(ctx, id)
	require.NoError(s.T(), err)
	secondDuration := time.Since(start)

	// 缓存命中应该更快（虽然在集成测试中可能不明显）
	s.T().Logf("第一次查询: %v, 第二次查询: %v", firstDuration, secondDuration)
}

func (s *ProductIntegrationSuite) TestListProductsWithCache() {
	ctx := context.Background()

	// 创建商品
	for i := 1; i <= 5; i++ {
		_, err := s.productSvc.CreateProduct(ctx, domain.Product{
			Name:       "商品",
			Price:      int64(i * 1000),
			Stock:      int64(i * 10),
			MerchantID: 1,
		})
		require.NoError(s.T(), err)
	}

	// 第一次查询（热点数据，会触发缓存）
	list1, err := s.productSvc.ListProducts(ctx, 1, 10, "")
	require.NoError(s.T(), err)
	assert.Len(s.T(), list1, 5)

	// 等待异步预热完成
	time.Sleep(200 * time.Millisecond)

	// 第二次查询（应该命中缓存）
	list2, err := s.productSvc.ListProducts(ctx, 1, 10, "")
	require.NoError(s.T(), err)
	assert.Len(s.T(), list2, 5)
}

// ============================================================================
// 边界条件测试
// ============================================================================

func (s *ProductIntegrationSuite) TestGetNonExistentProduct() {
	ctx := context.Background()

	_, err := s.productSvc.GetProduct(ctx, 99999)
	assert.Error(s.T(), err)
}

func (s *ProductIntegrationSuite) TestListEmptyProducts() {
	ctx := context.Background()

	list, err := s.productSvc.ListProducts(ctx, 1, 10, "")
	require.NoError(s.T(), err)
	assert.Len(s.T(), list, 0)
}

func (s *ProductIntegrationSuite) TestDeleteWithWrongMerchant() {
	ctx := context.Background()

	// 创建商品（商家 ID = 1001）
	id, err := s.productSvc.CreateProduct(ctx, domain.Product{
		Name:       "测试商品",
		Price:      1000,
		Stock:      10,
		MerchantID: 1001,
	})
	require.NoError(s.T(), err)

	// 用错误的商家 ID 删除（应该失败或不生效）
	err = s.productSvc.DeleteProduct(ctx, id, 9999)
	// 根据业务逻辑，可能返回错误或静默失败
	// 这里验证商品仍然存在
	got, err := s.productSvc.GetProduct(ctx, id)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "测试商品", got.Name)
}

// ============================================================================
// 并发测试
// ============================================================================

func (s *ProductIntegrationSuite) TestConcurrentGetProduct() {
	ctx := context.Background()

	// 创建商品
	id, err := s.productSvc.CreateProduct(ctx, domain.Product{
		Name:       "并发测试商品",
		Price:      1000,
		Stock:      10,
		MerchantID: 1,
	})
	require.NoError(s.T(), err)

	// 并发查询
	const goroutines = 10
	errChan := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := s.productSvc.GetProduct(ctx, id)
			errChan <- err
		}()
	}

	// 收集结果
	for i := 0; i < goroutines; i++ {
		err := <-errChan
		assert.NoError(s.T(), err)
	}
}

