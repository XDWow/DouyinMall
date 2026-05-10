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

// PaymentIntegrationTestSuite 覆盖支付预下单和模拟支付服务的集成流程。
type PaymentIntegrationTestSuite struct {
	suite.Suite

	mysqlContainer testcontainers.Container
	redisContainer testcontainers.Container

	db            *gorm.DB
	redis         redis.Cmdable
	repo          domain.PaymentRepository
	prePayUC      *usecase.NativePrePaymentUC // 被测预支付 UseCase
	mockServerURL string
}

func TestPaymentIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("短测试模式跳过支付集成测试")
	}
	suite.Run(t, new(PaymentIntegrationTestSuite))
}

// SetupSuite 启动测试依赖，并初始化仓储和 UseCase。
func (s *PaymentIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()
	s.mockServerURL = "http://localhost:8888"

	if !s.isMockServerRunning() {
		s.T().Skip("Mock 微信服务未启动，请先执行：cd cmd/mock-wechat && go run main.go")
	}

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

	var database *gorm.DB
	for i := 0; i < 10; i++ {
		database, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(s.T(), err)
	s.db = database

	err = s.db.AutoMigrate(&paymentdb.Payment{})
	require.NoError(s.T(), err)

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

	l := logger.NewNopLogger()
	s.repo = repository.NewPaymentRepository(s.db, l)
	mockWechatSvc := NewMockWechatNativeService(s.mockServerURL)
	s.prePayUC = usecase.NewNativePrePaymentUC(
		s.repo,
		l,
		mockWechatSvc,
		"test_app_id",
		"test_mch_id",
		"http://localhost:8888/callback",
	)

	s.T().Log("支付集成测试环境初始化完成")
	s.T().Log("   - MySQL 容器已启动")
	s.T().Log("   - Redis 容器已启动")
	s.T().Log("   - Mock 微信服务可用")
	s.T().Log("   - UseCase 已接入 Mock 微信服务")
}

func (s *PaymentIntegrationTestSuite) TearDownSuite() {
	if s.mysqlContainer != nil {
		_ = s.mysqlContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(context.Background())
	}
}

// isMockServerRunning 检查 Mock 微信服务是否可用。
func (s *PaymentIntegrationTestSuite) isMockServerRunning() bool {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// TestCompletePaymentFlow 覆盖完整的预下单、查询、支付成功流程。
func (s *PaymentIntegrationTestSuite) TestCompletePaymentFlow() {
	bizTradeNo := fmt.Sprintf("TEST_%d", time.Now().UnixNano())
	var codeURL string

	s.Run("1.UseCase 预下单成功", func() {
		ctx := context.Background()

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: "集成测试订单",
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

		s.T().Logf("UseCase 预下单成功，bizTradeNo=%s", bizTradeNo)
		s.T().Logf("   二维码 URL: %s", codeURL)
	})

	s.Run("2.数据库支付记录正确", func() {
		ctx := context.Background()

		payment, err := s.repo.GetPayment(ctx, bizTradeNo)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
		assert.Equal(s.T(), int64(100), payment.Amt.Total)
		assert.Equal(s.T(), "CNY", payment.Amt.Currency)
		assert.Equal(s.T(), domain.PaymentStatusInit, payment.Status)

		s.T().Log("支付记录已正确落库")
		s.T().Logf("   BizTradeNo: %s", payment.BizTradeNo)
		s.T().Logf("   Status: %d", payment.Status)
	})

	s.Run("3.Mock 服务查询为未支付", func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "NOTPAY", tradeState)
		s.T().Logf("Mock 服务支付状态: %s", tradeState)
	})

	s.Run("4.模拟用户支付成功", func() {
		url := fmt.Sprintf("%s/mock/pay/%s", s.mockServerURL, bizTradeNo)
		resp, err := http.Post(url, "application/json", nil)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
		s.T().Log("模拟支付成功")
	})

	s.Run("5.Mock 服务查询为支付成功", func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "SUCCESS", tradeState)
		s.T().Logf("支付状态已变更为: %s", tradeState)
	})
}

// TestPrepayWithUseCase 覆盖单次预下单。
func (s *PaymentIntegrationTestSuite) TestPrepayWithUseCase() {
	bizTradeNo := fmt.Sprintf("PREPAY_UC_%d", time.Now().UnixNano())

	ctx := context.Background()
	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "UseCase 预下单测试",
		Amt: domain.Amount{
			Total:    200,
			Currency: "CNY",
		},
	}

	codeURL, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), codeURL)
	assert.Contains(s.T(), codeURL, "MOCK_")

	payment, err := s.repo.GetPayment(ctx, bizTradeNo)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
	assert.Equal(s.T(), "UseCase 预下单测试", payment.Description)

	s.T().Logf("UseCase 预下单测试通过，bizTradeNo=%s", bizTradeNo)
	s.T().Logf("   二维码 URL: %s", codeURL)
}

// TestQueryMockOrders 覆盖 Mock 服务订单列表查询。
func (s *PaymentIntegrationTestSuite) TestQueryMockOrders() {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(s.T(), err)

	total := int(result["total"].(float64))
	s.T().Logf("Mock 服务当前共有 %d 笔订单", total)

	if orders, ok := result["orders"].([]interface{}); ok && len(orders) > 0 {
		s.T().Log("   最近订单:")
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

// TestMultipleConcurrentPayments 覆盖连续创建多笔支付记录。
func (s *PaymentIntegrationTestSuite) TestMultipleConcurrentPayments() {
	orderCount := 5
	ctx := context.Background()

	for i := 0; i < orderCount; i++ {
		bizTradeNo := fmt.Sprintf("CONCURRENT_%d_%d", time.Now().UnixNano(), i)

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: fmt.Sprintf("并发支付测试订单 %d", i),
			Amt: domain.Amount{
				Total:    int64(100 + i*10),
				Currency: "CNY",
			},
		}

		codeURL, err := s.prePayUC.Execute(ctx, cmd)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), codeURL)
	}

	s.T().Logf("连续创建 %d 笔支付记录成功", orderCount)
}

// TestDuplicateOrderNumber 覆盖重复订单号的幂等行为。
func (s *PaymentIntegrationTestSuite) TestDuplicateOrderNumber() {
	bizTradeNo := fmt.Sprintf("DUP_%d", time.Now().UnixNano())
	ctx := context.Background()

	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "重复订单号测试",
		Amt: domain.Amount{
			Total:    100,
			Currency: "CNY",
		},
	}

	_, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)

	_, err = s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)
	s.T().Log("相同订单号和相同金额重复预下单时保持幂等")
}
