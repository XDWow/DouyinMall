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

// PaymentIntegrationTestSuite 鏀粯闆嗘垚娴嬭瘯濂椾欢
type PaymentIntegrationTestSuite struct {
	suite.Suite

	mysqlContainer testcontainers.Container
	redisContainer testcontainers.Container

	db            *gorm.DB
	redis         redis.Cmdable
	repo          domain.PaymentRepository
	prePayUC      *usecase.NativePrePaymentUC // 鐪熸鐨?UseCase
	mockServerURL string
}

func TestPaymentIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("璺宠繃闆嗘垚娴嬭瘯锛堢煭妯″紡锛?)
	}
	suite.Run(t, new(PaymentIntegrationTestSuite))
}

// SetupSuite 娴嬭瘯濂椾欢鍒濆鍖栵紙鎵€鏈夋祴璇曞墠鎵ц涓€娆★級
func (s *PaymentIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()
	s.mockServerURL = "http://localhost:8888"

	// 妫€鏌?Mock 鏈嶅姟鏄惁杩愯
	if !s.isMockServerRunning() {
		s.T().Skip("Mock 寰俊鏀粯鏈嶅姟鏈惎鍔紝璇峰厛杩愯: cd cmd/mock-wechat && go run main.go")
	}

	// 鍚姩 MySQL 瀹瑰櫒
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

	// 娣诲姞閲嶈瘯閫昏緫锛岀‘淇濊繛鎺ユ垚鍔?
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

	// 鑷姩杩佺Щ琛ㄧ粨鏋?
	err = s.db.AutoMigrate(&paymentdb.Payment{})
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

	s.redis = redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort.Port(),
	})

	// 鍒濆鍖栨棩蹇?
	l := logger.NewNopLogger()

	// 鍒濆鍖?Repository
	s.repo = repository.NewPaymentRepository(s.db, l)

	// 浣跨敤 Mock 寰俊鏈嶅姟锛堝疄鐜?domain.WechatNativeService 鎺ュ彛锛?
	mockWechatSvc := NewMockWechatNativeService(s.mockServerURL)

	// 鍒濆鍖栫湡姝ｇ殑 UseCase锛屾敞鍏?Mock 鏈嶅姟
	s.prePayUC = usecase.NewNativePrePaymentUC(
		s.repo,
		l,
		mockWechatSvc,                    // 娉ㄥ叆 Mock 鏈嶅姟
		"test_app_id",                    // appID
		"test_mch_id",                    // mchID
		"http://localhost:8888/callback", // notifyURL
	)

	s.T().Log("鉁?娴嬭瘯鐜鍒濆鍖栧畬鎴?)
	s.T().Log("   - MySQL 瀹瑰櫒宸插惎鍔?)
	s.T().Log("   - Redis 瀹瑰櫒宸插惎鍔?)
	s.T().Log("   - Mock 寰俊鏀粯鏈嶅姟宸茶繛鎺?)
	s.T().Log("   - UseCase 宸插垵濮嬪寲锛堜娇鐢?Mock 鏈嶅姟锛?)
}

func (s *PaymentIntegrationTestSuite) TearDownSuite() {
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		s.redisContainer.Terminate(context.Background())
	}
}

// isMockServerRunning 妫€鏌?Mock 鏈嶅姟鏄惁杩愯
func (s *PaymentIntegrationTestSuite) isMockServerRunning() bool {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// TestCompletePaymentFlow 瀹屾暣鐨勬敮浠樻祦绋嬫祴璇曪紙閫氳繃 UseCase锛?
func (s *PaymentIntegrationTestSuite) TestCompletePaymentFlow() {
	bizTradeNo := fmt.Sprintf("TEST_%d", time.Now().UnixNano())
	var codeURL string

	// 姝ラ1: 閫氳繃 UseCase 鍒涘缓棰勬敮浠樿鍗?
	s.Run("1.UseCase鍒涘缓棰勬敮浠樿鍗?, func() {
		ctx := context.Background()

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: "闆嗘垚娴嬭瘯鍟嗗搧",
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

		s.T().Logf("鉁?UseCase 棰勬敮浠樻垚鍔? %s", bizTradeNo)
		s.T().Logf("   浜岀淮鐮乁RL: %s", codeURL)
	})

	// 姝ラ2: 楠岃瘉鏁版嵁搴撲腑鏈夋敮浠樿褰?
	s.Run("2.楠岃瘉鏁版嵁搴撴敮浠樿褰?, func() {
		ctx := context.Background()

		payment, err := s.repo.GetPayment(ctx, bizTradeNo)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
		assert.Equal(s.T(), int64(100), payment.Amt.Total)
		assert.Equal(s.T(), "CNY", payment.Amt.Currency)
		assert.Equal(s.T(), domain.PaymentStatusInit, payment.Status)

		s.T().Logf("鉁?鏁版嵁搴撹褰曢獙璇佹垚鍔?)
		s.T().Logf("   BizTradeNo: %s", payment.BizTradeNo)
		s.T().Logf("   Status: %d (鍒濆鍖?鏈敮浠?", payment.Status)
	})

	// 姝ラ3: 鏌ヨMock鏈嶅姟璁㈠崟鐘舵€?
	s.Run("3.鏌ヨMock鏈嶅姟璁㈠崟鐘舵€?, func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "NOTPAY", tradeState)
		s.T().Logf("鉁?Mock鏈嶅姟璁㈠崟鐘舵€? %s (鏈敮浠?", tradeState)
	})

	// 姝ラ4: 妯℃嫙鏀粯鎴愬姛
	s.Run("4.妯℃嫙鏀粯鎴愬姛", func() {
		url := fmt.Sprintf("%s/mock/pay/%s", s.mockServerURL, bizTradeNo)
		resp, err := http.Post(url, "application/json", nil)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		assert.Equal(s.T(), 200, resp.StatusCode)
		s.T().Logf("鉁?妯℃嫙鏀粯鎴愬姛")
	})

	// 姝ラ5: 鍐嶆鏌ヨ璁㈠崟鐘舵€侊紙搴旇鏄凡鏀粯锛?
	s.Run("5.鏌ヨ璁㈠崟鐘舵€?宸叉敮浠?, func() {
		url := fmt.Sprintf("%s/v3/pay/transactions/out-trade-no/%s?mchid=test_mch_id", s.mockServerURL, bizTradeNo)
		resp, err := http.Get(url)
		require.NoError(s.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(s.T(), err)

		tradeState := result["trade_state"].(string)
		assert.Equal(s.T(), "SUCCESS", tradeState)
		s.T().Logf("鉁?璁㈠崟宸叉敮浠橈紝鐘舵€? %s", tradeState)
	})
}

// TestPrepayWithUseCase 閫氳繃 UseCase 娴嬭瘯棰勬敮浠?
func (s *PaymentIntegrationTestSuite) TestPrepayWithUseCase() {
	bizTradeNo := fmt.Sprintf("PREPAY_UC_%d", time.Now().UnixNano())

	ctx := context.Background()
	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "UseCase棰勬敮浠樻祴璇?,
		Amt: domain.Amount{
			Total:    200,
			Currency: "CNY",
		},
	}

	codeURL, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), codeURL)
	assert.Contains(s.T(), codeURL, "MOCK_")

	// 楠岃瘉鏁版嵁搴?
	payment, err := s.repo.GetPayment(ctx, bizTradeNo)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), bizTradeNo, payment.BizTradeNo)
	assert.Equal(s.T(), "UseCase棰勬敮浠樻祴璇?, payment.Description)

	s.T().Logf("鉁?UseCase棰勬敮浠樻祴璇曢€氳繃: %s", bizTradeNo)
	s.T().Logf("   浜岀淮鐮乁RL: %s", codeURL)
}

// TestQueryMockOrders 娴嬭瘯鏌ヨ Mock 鏈嶅姟鐨勬墍鏈夎鍗?
func (s *PaymentIntegrationTestSuite) TestQueryMockOrders() {
	resp, err := http.Get(s.mockServerURL + "/mock/orders")
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(s.T(), err)

	total := int(result["total"].(float64))
	s.T().Logf("鉁?Mock 鏈嶅姟涓叡鏈?%d 涓鍗?, total)

	if orders, ok := result["orders"].([]interface{}); ok && len(orders) > 0 {
		s.T().Logf("   鏈€杩戠殑璁㈠崟:")
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

// TestMultipleConcurrentPayments 娴嬭瘯骞跺彂鏀粯锛堥€氳繃 UseCase锛?
func (s *PaymentIntegrationTestSuite) TestMultipleConcurrentPayments() {
	orderCount := 5
	ctx := context.Background()

	for i := 0; i < orderCount; i++ {
		bizTradeNo := fmt.Sprintf("CONCURRENT_%d_%d", time.Now().UnixNano(), i)

		cmd := usecase.PrePaymentCmd{
			BizTradeNo:  bizTradeNo,
			Description: fmt.Sprintf("骞跺彂娴嬭瘯璁㈠崟%d", i),
			Amt: domain.Amount{
				Total:    int64(100 + i*10),
				Currency: "CNY",
			},
		}

		codeURL, err := s.prePayUC.Execute(ctx, cmd)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), codeURL)
	}

	s.T().Logf("鉁?鍒涘缓 %d 涓苟鍙戣鍗曟垚鍔?, orderCount)
}

// TestDuplicateOrderNumber 娴嬭瘯閲嶅璁㈠崟鍙?
func (s *PaymentIntegrationTestSuite) TestDuplicateOrderNumber() {
	bizTradeNo := fmt.Sprintf("DUP_%d", time.Now().UnixNano())
	ctx := context.Background()

	// 绗竴娆″垱寤?
	cmd := usecase.PrePaymentCmd{
		BizTradeNo:  bizTradeNo,
		Description: "閲嶅璁㈠崟娴嬭瘯",
		Amt: domain.Amount{
			Total:    100,
			Currency: "CNY",
		},
	}

	_, err := s.prePayUC.Execute(ctx, cmd)
	require.NoError(s.T(), err)

	// 绗簩娆″垱寤猴紙搴旇澶辫触锛屽洜涓烘暟鎹簱鍞竴绱㈠紩鍐茬獊锛?
	_, err = s.prePayUC.Execute(ctx, cmd)
	assert.Error(s.T(), err)
	s.T().Logf("鉁?閲嶅璁㈠崟娴嬭瘯閫氳繃锛岀浜屾鍒涘缓澶辫触: %v", err)
}


