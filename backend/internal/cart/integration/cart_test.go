//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/cart/handler"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/cache"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/dao"
	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	cartv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type CartIntegrationSuite struct {
	suite.Suite

	mysqlContainer testcontainers.Container
	redisContainer testcontainers.Container

	db    *gorm.DB
	redis redis.Cmdable

	cartHandler *handler.CartHandler
}

func TestCartIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(CartIntegrationSuite))
}

func (s *CartIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	mysqlReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "douyinmall_test",
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

	dsn := "root:root@tcp(" + mysqlHost + ":" + mysqlPort.Port() + ")/douyinmall_test?charset=utf8mb4&parseTime=True&loc=Local"

	var db *gorm.DB
	for i := 0; i < 5; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.Ping() == nil {
				break
			}
		}
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(s.T(), err)
	s.db = db
	require.NoError(s.T(), db.AutoMigrate(&dao.CartItem{}))

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

	cartDAO := dao.NewGORMCartDAO(db)
	cartCache := cache.NewRedisCache(redisClient)
	cartRepo := repository.NewCachedCartRepository(cartCache, cartDAO, logger.NewNopLogger())
	s.cartHandler = handler.NewCartHandler(service.NewCartService(cartRepo))
}

func (s *CartIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.mysqlContainer != nil {
		_ = s.mysqlContainer.Terminate(ctx)
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(ctx)
	}
}

func (s *CartIntegrationSuite) SetupTest() {
	ctx := context.Background()
	s.db.Exec("TRUNCATE TABLE cart_items")
	s.redis.FlushDB(ctx)
}

func (s *CartIntegrationSuite) TestAddItem() {
	ctx := context.Background()

	resp, err := s.cartHandler.AddItem(ctx, addItemReq(1001, 2001, 3001, 1))
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), qty)
	productID, err := s.redis.HGet(ctx, "cart:1001:products", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2001), productID)

	time.Sleep(100 * time.Millisecond)

	var item dao.CartItem
	err = s.db.Where("user_id = ? AND sku_id = ?", 1001, 3001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2001), item.ProductID)
	assert.Equal(s.T(), int64(1), item.Quantity)
}

func (s *CartIntegrationSuite) TestAddItemDuplicateIncrementsSameSKU() {
	ctx := context.Background()

	_, err := s.cartHandler.AddItem(ctx, addItemReq(1001, 2001, 3001, 1))
	require.NoError(s.T(), err)
	_, err = s.cartHandler.AddItem(ctx, addItemReq(1001, 2001, 3001, 1))
	require.NoError(s.T(), err)

	time.Sleep(100 * time.Millisecond)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)

	var item dao.CartItem
	err = s.db.Where("user_id = ? AND sku_id = ?", 1001, 3001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), item.Quantity)
}

func (s *CartIntegrationSuite) TestGetCart() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2002, 3002, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.GetCart(ctx, &cartv1.GetCartReq{UserId: 1001})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp.GetCart())
	assert.Equal(s.T(), int64(1001), resp.GetCart().GetUserId())
	assert.Len(s.T(), resp.GetCart().GetItems(), 2)

	skuIDs := make(map[int64]bool)
	for _, item := range resp.GetCart().GetItems() {
		skuIDs[item.GetSkuId()] = true
	}
	assert.True(s.T(), skuIDs[3001])
	assert.True(s.T(), skuIDs[3002])
}

func (s *CartIntegrationSuite) TestGetCartEmpty() {
	ctx := context.Background()

	resp, err := s.cartHandler.GetCart(ctx, &cartv1.GetCartReq{UserId: 9999})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp.GetCart())
	assert.Equal(s.T(), int64(9999), resp.GetCart().GetUserId())
	assert.Empty(s.T(), resp.GetCart().GetItems())
}

func (s *CartIntegrationSuite) TestDeleteItem() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.DeleteItem(ctx, &cartv1.DeleteItemReq{
		UserId: 1001,
		SkuIds: []int64{3001},
	})
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	time.Sleep(100 * time.Millisecond)

	exists, err := s.redis.HExists(ctx, "cart:1001", "3001").Result()
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
	exists, err = s.redis.HExists(ctx, "cart:1001:products", "3001").Result()
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	var count int64
	s.db.Model(&dao.CartItem{}).Where("user_id = ? AND sku_id = ?", 1001, 3001).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

func (s *CartIntegrationSuite) TestChangeQty() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.ChangeQty(ctx, &cartv1.ChangeQtyReq{
		UserId: 1001,
		Item:   cartItem(2001, 3001, 5),
	})
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	time.Sleep(100 * time.Millisecond)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), qty)

	var item dao.CartItem
	err = s.db.Where("user_id = ? AND sku_id = ?", 1001, 3001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), item.Quantity)
}

func (s *CartIntegrationSuite) TestIncrementQty() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.IncrementQty(ctx, &cartv1.IncrementQtyReq{
		UserId: 1001,
		SkuId:  3001,
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(2), resp.GetNewQuantity())

	time.Sleep(100 * time.Millisecond)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)
}

func (s *CartIntegrationSuite) TestDecrementQty() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 3))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.DecrementQty(ctx, &cartv1.DecrementQtyReq{
		UserId: 1001,
		SkuId:  3001,
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(2), resp.GetNewQuantity())

	time.Sleep(100 * time.Millisecond)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)
}

func (s *CartIntegrationSuite) TestDecrementQtyMinQuantity() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.DecrementQty(ctx, &cartv1.DecrementQtyReq{
		UserId: 1001,
		SkuId:  3001,
	})
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "cannot be decremented")
}

func (s *CartIntegrationSuite) TestEmptyCart() {
	ctx := context.Background()

	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2001, 3001, 1))
	require.NoError(s.T(), addCartItem(ctx, s.cartHandler, 1001, 2002, 3002, 1))
	time.Sleep(100 * time.Millisecond)

	resp, err := s.cartHandler.EmptyCart(ctx, &cartv1.EmptyCartReq{UserId: 1001})
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	time.Sleep(100 * time.Millisecond)

	exists, err := s.redis.Exists(ctx, "cart:1001", "cart:1001:products").Result()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), exists)

	var count int64
	s.db.Model(&dao.CartItem{}).Where("user_id = ?", 1001).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

func (s *CartIntegrationSuite) TestGetCartRedisMissLoadFromMySQL() {
	ctx := context.Background()

	require.NoError(s.T(), s.db.Create(&dao.CartItem{
		UserID:    1001,
		ProductID: 2001,
		SKUID:     3001,
		Quantity:  3,
	}).Error)
	require.NoError(s.T(), s.redis.Del(ctx, "cart:1001", "cart:1001:products").Err())

	resp, err := s.cartHandler.GetCart(ctx, &cartv1.GetCartReq{UserId: 1001})
	require.NoError(s.T(), err)
	require.Len(s.T(), resp.GetCart().GetItems(), 1)
	assert.Equal(s.T(), int64(3001), resp.GetCart().GetItems()[0].GetSkuId())
	assert.Equal(s.T(), int64(3), resp.GetCart().GetItems()[0].GetQuantity())

	time.Sleep(100 * time.Millisecond)

	qty, err := s.redis.HGet(ctx, "cart:1001", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), qty)
	productID, err := s.redis.HGet(ctx, "cart:1001:products", "3001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2001), productID)
}

func addItemReq(userID, productID, skuID, quantity int64) *cartv1.AddItemReq {
	return &cartv1.AddItemReq{
		UserId: userID,
		Items:  []*cartv1.CartItem{cartItem(productID, skuID, quantity)},
	}
}

func cartItem(productID, skuID, quantity int64) *cartv1.CartItem {
	return &cartv1.CartItem{
		ProductId: productID,
		SkuId:     skuID,
		Quantity:  quantity,
	}
}

func addCartItem(ctx context.Context, h *handler.CartHandler, userID, productID, skuID, quantity int64) error {
	_, err := h.AddItem(ctx, addItemReq(userID, productID, skuID, quantity))
	return err
}
