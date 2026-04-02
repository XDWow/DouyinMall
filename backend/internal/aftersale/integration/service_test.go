//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	aftersaledb "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/db"
	aftersalerepository "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/repository"
	aftersalegrpc "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/grpc"
	aftersaleusecase "github.com/XDWow/DouyinMall/backend/internal/aftersale/usecase"
	aftersalev1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type AfterSaleIntegrationSuite struct {
	suite.Suite

	mysqlContainer testcontainers.Container
	db             *gorm.DB
	handler        *aftersalegrpc.Handler
}

func TestAfterSaleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests in short mode")
	}
	suite.Run(t, new(AfterSaleIntegrationSuite))
}

func (s *AfterSaleIntegrationSuite) SetupSuite() {
	ctx := context.Background()
	requireDockerDaemon(s.T())

	mysqlReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "aftersale_test",
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

	dsn := fmt.Sprintf(
		"root:root@tcp(%s:%s)/aftersale_test?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlHost,
		mysqlPort.Port(),
	)

	var db *gorm.DB
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr == nil && sqlDB.Ping() == nil {
				break
			}
			err = sqlErr
		}
		if i < 9 {
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(s.T(), err)
	s.db = db

	require.NoError(s.T(), aftersaledb.InitTables(db))

	repo := aftersalerepository.NewRequestRepository(db)
	createRequestUC := aftersaleusecase.NewCreateAfterSaleRequestUseCase(repo)
	getRequestUC := aftersaleusecase.NewGetAfterSaleRequestUseCase(repo)
	s.handler = aftersalegrpc.NewHandler(createRequestUC, getRequestUC)
}

func (s *AfterSaleIntegrationSuite) TearDownSuite() {
	if s.mysqlContainer != nil {
		_ = s.mysqlContainer.Terminate(context.Background())
	}
}

func (s *AfterSaleIntegrationSuite) SetupTest() {
	s.db.Exec("TRUNCATE TABLE after_sale_requests")
}

func (s *AfterSaleIntegrationSuite) TestGetAfterSaleRequest() {
	ctx := context.Background()

	createResp, err := s.handler.CreateAfterSaleRequest(ctx, &aftersalev1.CreateAfterSaleRequestReq{
		UserId:       1001,
		OrderId:      2002,
		ItemId:       3003,
		RequestType:  aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_RETURN,
		Reason:       "item damaged",
		SessionId:    "sess-001",
		TraceId:      "trace-001",
		MetadataJson: "{\"source\":\"integration\"}",
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createResp)
	require.NotNil(s.T(), createResp.Request)

	getResp, err := s.handler.GetAfterSaleRequest(ctx, &aftersalev1.GetAfterSaleRequestReq{
		RequestNo: createResp.Request.RequestNo,
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), getResp)
	require.NotNil(s.T(), getResp.Request)

	assert.Equal(s.T(), createResp.Request.RequestNo, getResp.Request.RequestNo)
	assert.Equal(s.T(), int64(1001), getResp.Request.UserId)
	assert.Equal(s.T(), int64(2002), getResp.Request.OrderId)
	assert.Equal(s.T(), int64(3003), getResp.Request.ItemId)
	assert.Equal(s.T(), aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_RETURN, getResp.Request.RequestType)
	assert.Equal(s.T(), "item damaged", getResp.Request.Reason)
	assert.Equal(s.T(), aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_PENDING_REVIEW, getResp.Request.Status)
	assert.NotZero(s.T(), getResp.Request.CreatedAt)
	assert.WithinDuration(s.T(), time.Now(), time.Unix(getResp.Request.CreatedAt, 0), 2*time.Minute)
}

func requireDockerDaemon(t *testing.T) {
	t.Helper()

	cmd := exec.Command("docker", "version")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skip integration test: docker daemon unavailable: %v (%s)", err, string(output))
	}
}
