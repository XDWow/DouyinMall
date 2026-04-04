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
		t.Skip("璺宠繃闆嗘垚娴嬭瘯锛堢煭妯″紡锛?)
	}
	suite.Run(t, new(CartIntegrationSuite))
}

func (s *CartIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 鍚姩 MySQL 瀹瑰櫒
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

	// 娣诲姞閲嶈瘯閫昏緫锛岀‘淇濊繛鎺ユ垚鍔?
	var db *gorm.DB
	for i := 0; i < 5; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			// 娴嬭瘯杩炴帴
			sqlDB, err := db.DB()
			if err == nil {
				err = sqlDB.Ping()
				if err == nil {
					break
				}
			}
		}
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(s.T(), err)
	s.db = db

	err = db.AutoMigrate(&dao.CartItem{})
	require.NoError(s.T(), err)

	// 鍚姩 Redis 瀹瑰櫒
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

	// 鍒濆鍖栨湇鍔＄粍浠?
	cartDAO := dao.NewGORMCartDAO(db)
	cartCache := cache.NewRedisCache(redisClient)
	testLogger := logger.NewNopLogger()
	cartRepo := repository.NewCachedCartRepository(cartCache, cartDAO, testLogger)
	cartSvc := service.NewCartService(cartRepo)
	s.cartHandler = handler.NewCartHandler(cartSvc)
}

func (s *CartIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(ctx)
	}
	if s.redisContainer != nil {
		s.redisContainer.Terminate(ctx)
	}
}

func (s *CartIntegrationSuite) SetupTest() {
	ctx := context.Background()
	s.db.Exec("TRUNCATE TABLE cart_items")
	s.redis.FlushDB(ctx)
}

func (s *CartIntegrationSuite) TestAddItem() {
	ctx := context.Background()

	req := &cartv1.AddItemReq{
		UserId:     1001,
		ProductIds: []int64{2001},
	}

	resp, err := s.cartHandler.AddItem(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// 楠岃瘉 Redis
	key := "cart:1001"
	result, err := s.redis.HGetAll(ctx, key).Result()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "1", result["2001"])

	// 绛夊緟寮傛鍐?MySQL 瀹屾垚
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉 MySQL
	var item dao.CartItem
	err = s.db.Where("user_id = ? AND product_id = ?", 1001, 2001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), item.Quantity)
}

func (s *CartIntegrationSuite) TestAddItem_Duplicate() {
	ctx := context.Background()

	req := &cartv1.AddItemReq{
		UserId:     1001,
		ProductIds: []int64{2001},
	}

	// 绗竴娆℃坊鍔?
	_, err := s.cartHandler.AddItem(ctx, req)
	require.NoError(s.T(), err)

	// 绗簩娆℃坊鍔狅紙搴旇绱姞锛?
	_, err = s.cartHandler.AddItem(ctx, req)
	require.NoError(s.T(), err)

	// 绛夊緟寮傛鍐?MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉鏁伴噺鏄?2
	key := "cart:1001"
	qty, err := s.redis.HGet(ctx, key, "2001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)

	var item dao.CartItem
	err = s.db.Where("user_id = ? AND product_id = ?", 1001, 2001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), item.Quantity)
}

func (s *CartIntegrationSuite) TestGetCart() {
	ctx := context.Background()

	// 鍏堟坊鍔犲嚑涓晢鍝?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2002}})
	time.Sleep(100 * time.Millisecond)

	req := &cartv1.GetCartReq{
		UserId: 1001,
	}

	resp, err := s.cartHandler.GetCart(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(1001), resp.Cart.UserId)
	assert.Len(s.T(), resp.Cart.Items, 2)

	productIDs := make(map[int64]bool)
	for _, item := range resp.Cart.Items {
		productIDs[item.ProductId] = true
	}
	assert.True(s.T(), productIDs[2001])
	assert.True(s.T(), productIDs[2002])
}

func (s *CartIntegrationSuite) TestGetCart_Empty() {
	ctx := context.Background()

	req := &cartv1.GetCartReq{
		UserId: 9999,
	}

	resp, err := s.cartHandler.GetCart(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(9999), resp.Cart.UserId)
	assert.Len(s.T(), resp.Cart.Items, 0)
}

func (s *CartIntegrationSuite) TestDeleteItem() {
	ctx := context.Background()

	// 鍏堟坊鍔犲晢鍝?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	time.Sleep(100 * time.Millisecond)

	// 鍒犻櫎鍟嗗搧
	req := &cartv1.DeleteItemReq{
		UserId:     1001,
		ProductIds: []int64{2001},
	}

	resp, err := s.cartHandler.DeleteItem(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// 绛夊緟寮傛鍒犻櫎 MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉 Redis 涓凡鍒犻櫎
	key := "cart:1001"
	exists, err := s.redis.HExists(ctx, key, "2001").Result()
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	// 楠岃瘉 MySQL 涓凡鍒犻櫎
	var count int64
	s.db.Model(&dao.CartItem{}).Where("user_id = ? AND product_id = ?", 1001, 2001).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

func (s *CartIntegrationSuite) TestChangeQty() {
	ctx := context.Background()

	// 鍏堟坊鍔犲晢鍝?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	time.Sleep(100 * time.Millisecond)

	// 淇敼鏁伴噺涓?5
	req := &cartv1.ChangeQtyReq{
		UserId: 1001,
		Item: &cartv1.CartItem{
			ProductId: 2001,
			Quantity:  5,
		},
	}

	resp, err := s.cartHandler.ChangeQty(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// 绛夊緟寮傛鍐?MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉鏁伴噺鏄?5
	key := "cart:1001"
	qty, err := s.redis.HGet(ctx, key, "2001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), qty)

	var item dao.CartItem
	err = s.db.Where("user_id = ? AND product_id = ?", 1001, 2001).First(&item).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), item.Quantity)
}

func (s *CartIntegrationSuite) TestIncrementQty() {
	ctx := context.Background()

	// 鍏堟坊鍔犲晢鍝?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	time.Sleep(100 * time.Millisecond)

	// 澧炲姞鏁伴噺
	req := &cartv1.IncrementQtyReq{
		UserId:    1001,
		ProductId: 2001,
	}

	resp, err := s.cartHandler.IncrementQty(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(2), resp.NewQuantity)

	// 绛夊緟寮傛鍐?MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉鏁伴噺鏄?2
	key := "cart:1001"
	qty, err := s.redis.HGet(ctx, key, "2001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)
}

func (s *CartIntegrationSuite) TestDecrementQty() {
	ctx := context.Background()

	// 鍏堟坊鍔犲晢鍝侊紝鏁伴噺璁句负 3
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	time.Sleep(100 * time.Millisecond)

	// 鍑忓皯鏁伴噺
	req := &cartv1.DecrementQtyReq{
		UserId:    1001,
		ProductId: 2001,
	}

	resp, err := s.cartHandler.DecrementQty(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(2), resp.NewQuantity)

	// 绛夊緟寮傛鍐?MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉鏁伴噺鏄?2
	key := "cart:1001"
	qty, err := s.redis.HGet(ctx, key, "2001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), qty)
}

func (s *CartIntegrationSuite) TestDecrementQty_MinQuantity() {
	ctx := context.Background()

	// 鍏堟坊鍔犲晢鍝侊紙鏁伴噺涓?1锛?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	time.Sleep(100 * time.Millisecond)

	// 灏濊瘯鍑忓皯鏁伴噺锛堝簲璇ュけ璐ワ級
	req := &cartv1.DecrementQtyReq{
		UserId:    1001,
		ProductId: 2001,
	}

	resp, err := s.cartHandler.DecrementQty(ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "涓嶈兘鍐嶅噺灏?)
}

func (s *CartIntegrationSuite) TestEmptyCart() {
	ctx := context.Background()

	// 鍏堟坊鍔犲嚑涓晢鍝?
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2001}})
	s.cartHandler.AddItem(ctx, &cartv1.AddItemReq{UserId: 1001, ProductIds: []int64{2002}})
	time.Sleep(100 * time.Millisecond)

	// 娓呯┖璐墿杞?
	req := &cartv1.EmptyCartReq{
		UserId: 1001,
	}

	resp, err := s.cartHandler.EmptyCart(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// 绛夊緟寮傛鍒犻櫎 MySQL
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉 Redis 宸叉竻绌?
	key := "cart:1001"
	exists, err := s.redis.Exists(ctx, key).Result()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), exists)

	// 楠岃瘉 MySQL 宸叉竻绌?
	var count int64
	s.db.Model(&dao.CartItem{}).Where("user_id = ?", 1001).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

func (s *CartIntegrationSuite) TestGetCart_RedisMiss_LoadFromMySQL() {
	ctx := context.Background()

	// 鐩存帴鍦?MySQL 鎻掑叆鏁版嵁锛堟ā鎷?Redis miss锛?
	item := dao.CartItem{
		UserID:    1001,
		ProductID: 2001,
		Quantity:  3,
	}
	s.db.Create(&item)

	// 娓呯┖ Redis锛堟ā鎷?miss锛?
	s.redis.Del(ctx, "cart:1001")

	// 鑾峰彇璐墿杞︼紙搴旇浠?MySQL 鍔犺浇骞跺洖鍐?Redis锛?
	req := &cartv1.GetCartReq{
		UserId: 1001,
	}

	resp, err := s.cartHandler.GetCart(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Len(s.T(), resp.Cart.Items, 1)
	assert.Equal(s.T(), int64(3), resp.Cart.Items[0].Quantity)

	// 绛夊緟寮傛鍥炲啓 Redis
	time.Sleep(100 * time.Millisecond)

	// 楠岃瘉 Redis 宸插洖鍐?
	key := "cart:1001"
	qty, err := s.redis.HGet(ctx, key, "2001").Int64()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), qty)
}


