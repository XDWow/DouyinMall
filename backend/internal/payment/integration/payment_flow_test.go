//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	paymentdb "github.com/XDWow/DouyinMall/backend/internal/payment/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
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

// PaymentIntegrationTestSuite 支付集成测试套件
type PaymentIntegrationTestSuite struct {
	suite.Suite

	mysqlContainer testcontainers.Container
	redisContainer testcontainers.Container

	db            *gorm.DB
	redis         redis.Cmdable
	repo          domain.PaymentRepository
	prePayUC      *usecase.NativePrePaymentUC // 真正的 UseCase
	mockServerURL string
}

func TestPaymentIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}
	suite.Run(t, new(PaymentIntegrationTestSuite))
}

// SetupSuite 测试套件初始化（所有测试前执行一次）
func (s *PaymentIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()
	s.mockServerURL = "http://localhost:8888"

	// 检查 Mock 服务是否运行
	if !s.isMockServerRunning() {
		s.T().Skip("Mock 微信支付服务未启动，请先运行: cd cmd/mock-wechat && go run main.go")
	}

	// 启动 MySQL 容器
	mysqlReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "payment_test",
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

	dsn := "root:root@tcp(" + mysqlHost + ":" + mysqlPort.Port() + ")/payment_test?charset=utf8mb4&parseTime=True&loc=Local"

	// 添加重试逻辑，确保连接成功
	var db *gorm.DB
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(s.T(), err)
	s.db = db

	// 自动迁移表结构
	err = s.db.AutoMigrate(&paymentdb.Payment{})
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

	s.redis = redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort.Port(),
	})

	// 初始化日志
	l := logger.NewNopLogger()

	// 初始化 Repository
	s.repo = repository.NewPaymentRepository(s.db, l)

	// 使用 Mock 微信服务（实现 domain.WechatNativeService 接口）
	mockWechatSvc := NewMockWechatNativeService(s.mockServerURL)

	// 初始化真正的 UseCase，注入 Mock 服务
	s.prePayUC = usecase.NewNativePrePaymentUC(
		s.repo,
		l,
		mockWechatSvc,                    // 注入 Mock 服务
		"test_app_id",                    // appID
		"test_mch_id",                    // mchID
		"http://localhost:8888/callback", // notifyURL
	)

	s.T().Log("✅ 测试环境初始化完成")
	s.T().Log("   - MySQL 容器已启动")
	s.T().Log("   - Redis 容器已启动")
	s.T().Log("   - Mock 微信支付服务已连接")
	s.T().Log("   - UseCase 已初始化（使用 Mock 服务）")
}

func (s *PaymentIntegrationTestSuite) TearDownSuite() {
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		s.redisContainer.Terminate(context.Background())
	}
}

// isMockServerRunning 检查 Mock 服务是否运行
func (s *PaymentIntegrationTestSuite) isMockServerRunning() bool {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// TestCompletePaymentFlow 完整的支付流程测试（通过 UseCase）
func (s *PaymentIntegrationTestSuite) TestCompletePaymentFlow() {
	bizTradeNo := fmt.Sprintf("TEST_%d", time.Now().UnixNano())
	var codeURL string

	// 步骤1: 通过 UseCase 创建预支付订单
	s.Run("1.UseCase创建预支付订单", func() {
		ctx := context.Background()

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: "集成测试商品",
			Amt: domain.Amount{
				Total:    100,
				Currency: "CNY",
			},
		}

		var err error
		codeURL, err = s.prePayUC.Execute(ctx, cmd)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), codeURL)
		assert.Contains(s.T(), codeURL, "MOCK_")

		s.T().Logf("✅ UseCase 预支付成功: %s", bizTradeNo)
		s.T().Logf("   二维码URL: %s", codeURL)
	})

	// 步骤2: 验证数据库中有支付记录
	s.Run("2.验证数据库支付记录", func() {
		ctx := context.Background()

		payment, err := s.repo.GetPayment(ctx, bizTradeNo)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
		assert.Equal(s.T(), int64(100), payment.Amt.Total)
		assert.Equal(s.T(), "CNY", payment.Amt.Currency)
		assert.Equal(s.T(), domain.PaymentStatusInit, payment.Status)

		s.T().Logf("✅ 数据库记录验证成功")
		s.T().Logf("   BizTradeNo: %s", payment.BizTradeNo)
		s.T().Logf("   Status: %d (初始化/未支付)", payment.Status)
	})

	// 步骤3: 查询Mock服务订单状态
	s.Run("3.查询Mock服务订单状态", func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "NOTPAY", tradeState)
		s.T().Logf("✅ Mock服务订单状态: %s (未支付)", tradeState)
	})

	// 步骤4: 模拟支付成功
	s.Run("4.模拟支付成功", func() {
		url := fmt.Sprintf("%s/mock/pay/%s", s.mockServerURL, bizTradeNo)
		resp, err := http.Post(url, "application/json", nil)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		assert.Equal(s.T(), 200, resp.StatusCode)
		s.T().Logf("✅ 模拟支付成功")
	})

	// 步骤5: 再次查询订单状态（应该是已支付）
	s.Run("5.查询订单状态-已支付", func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "SUCCESS", tradeState)
		s.T().Logf("✅ 订单已支付，状态: %s", tradeState)
	})
}

// TestPrepayWithUseCase 通过 UseCase 测试预支付
func (s *PaymentIntegrationTestSuite) TestPrepayWithUseCase() {
	bizTradeNo := fmt.Sprintf("PREPAY_UC_%d", time.Now().UnixNano())

	ctx := context.Background()
	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "UseCase预支付测试",
		Amt: domain.Amount{
			Total:    200,
			Currency: "CNY",
		},
	}

	codeURL, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), codeURL)
	assert.Contains(s.T(), codeURL, "MOCK_")

	// 验证数据库
	payment, err := s.repo.GetPayment(ctx, bizTradeNo)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
	assert.Equal(s.T(), "UseCase预支付测试", payment.Description)

	s.T().Logf("✅ UseCase预支付测试通过: %s", bizTradeNo)
	s.T().Logf("   二维码URL: %s", codeURL)
}

// TestQueryMockOrders 测试查询 Mock 服务的所有订单
func (s *PaymentIntegrationTestSuite) TestQueryMockOrders() {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(s.T(), err)

	total := int(result["total"].(float64))
	s.T().Logf("✅ Mock 服务中共有 %d 个订单", total)

	if orders, ok := result["orders"].([]interface{}); ok && len(orders) > 0 {
		s.T().Logf("   最近的订单:")
		for i, order := range orders {
			if i >= 5 {
				break
			}
			orderMap := order.(map[string]interface{})
			s.T().Logf("   - %s: %s",
				orderMap["out_trade_no"],
				orderMap["trade_state"])
		}
	}
}

// TestMultipleConcurrentPayments 测试并发支付（通过 UseCase）
func (s *PaymentIntegrationTestSuite) TestMultipleConcurrentPayments() {
	orderCount := 5
	ctx := context.Background()

	for i := 0; i < orderCount; i++ {
		bizTradeNo := fmt.Sprintf("CONCURRENT_%d_%d", time.Now().UnixNano(), i)

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: fmt.Sprintf("并发测试订单%d", i),
			Amt: domain.Amount{
				Total:    int64(100 + i*10),
				Currency: "CNY",
			},
		}

		codeURL, err := s.prePayUC.Execute(ctx, cmd)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), codeURL)
	}

	s.T().Logf("✅ 创建 %d 个并发订单成功", orderCount)
}

// TestDuplicateOrderNumber 测试重复订单号
func (s *PaymentIntegrationTestSuite) TestDuplicateOrderNumber() {
	bizTradeNo := fmt.Sprintf("DUP_%d", time.Now().UnixNano())
	ctx := context.Background()

	// 第一次创建
	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "重复订单测试",
		Amt: domain.Amount{
			Total:    100,
			Currency: "CNY",
		},
	}

	_, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)

	// 第二次创建（应该失败，因为数据库唯一索引冲突）
	_, err = s.prePayUC.Execute(ctx, cmd)
	assert.Error(s.T(), err)
	s.T().Logf("✅ 重复订单测试通过，第二次创建失败: %v", err)
}
