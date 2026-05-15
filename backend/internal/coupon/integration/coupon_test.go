//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	coupondb "github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCouponLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip coupon integration tests in short mode")
	}

	ctx := context.Background()
	mysqlContainer, database := setupCouponTestDB(t, ctx)
	defer func() {
		_ = mysqlContainer.Terminate(ctx)
	}()

	testLogger := logger.NewNopLogger()
	couponRepo := repository.NewCouponRepository(database, testLogger)
	templateRepo := repository.NewCouponTemplateRepository(database, testLogger)
	operationRepo := repository.NewCouponOperationRepository(database, testLogger)

	issueCouponUC := usecase.NewIssueCouponUseCase(templateRepo, couponRepo, operationRepo)
	reserveCouponUC := usecase.NewReserveCouponUseCase(couponRepo)
	commitCouponUC := usecase.NewCommitCouponUseCase(couponRepo)
	refundCouponUC := usecase.NewRefundCouponUseCase(couponRepo)

	templateID := createCouponTemplate(t, database)
	userID := int64(12345)
	orderID := int64(98765)

	issueOutput, err := issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      userID,
		TemplateID:  templateID,
		OperationID: "coupon-integration-issue-12345",
	})
	require.NoError(t, err)
	require.NotZero(t, issueOutput.CouponID)

	unusedCoupons, total, err := couponRepo.ListByUserID(ctx, userID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int32(1), total)
	require.Len(t, unusedCoupons, 1)
	require.Equal(t, issueOutput.CouponID, unusedCoupons[0].ID)

	reserveOutput, err := reserveCouponUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    userID,
		CouponIDs: []int64{issueOutput.CouponID},
		OrderID:   orderID,
	})
	require.NoError(t, err)
	require.True(t, reserveOutput.Success)
	require.Equal(t, 1, reserveOutput.ReservedCount)

	lockedCoupons, total, err := couponRepo.ListByUserID(ctx, userID, domain.UserCouponStatusLocked, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int32(1), total)
	require.Len(t, lockedCoupons, 1)
	require.Equal(t, orderID, lockedCoupons[0].OrderID)

	err = commitCouponUC.Execute(ctx, usecase.CommitCouponInput{OrderID: orderID})
	require.NoError(t, err)

	usedCoupons, total, err := couponRepo.ListByUserID(ctx, userID, domain.UserCouponStatusUsed, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int32(1), total)
	require.Len(t, usedCoupons, 1)

	err = refundCouponUC.Execute(ctx, usecase.RefundCouponInput{OrderID: orderID})
	require.NoError(t, err)

	unusedCoupons, total, err = couponRepo.ListByUserID(ctx, userID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int32(1), total)
	require.Len(t, unusedCoupons, 1)
	require.Equal(t, int64(0), unusedCoupons[0].OrderID)
}

func setupCouponTestDB(t *testing.T, ctx context.Context) (testcontainers.Container, *gorm.DB) {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mysql:8.0",
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "root",
				"MYSQL_DATABASE":      "coupon_test",
			},
			WaitingFor: wait.ForLog("ready for connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "3306")
	require.NoError(t, err)

	dsn := "root:root@tcp(" + host + ":" + port.Port() + ")/coupon_test?charset=utf8mb4&parseTime=True&loc=Local"

	var database *gorm.DB
	for i := 0; i < 10; i++ {
		database, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, sqlErr := database.DB()
			if sqlErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(t, err)

	err = coupondb.InitTables(database)
	require.NoError(t, err)

	return container, database
}

func createCouponTemplate(t *testing.T, database *gorm.DB) int64 {
	t.Helper()

	now := time.Now()
	minAmount := int32(10000)
	template := &coupondb.CouponTemplate{
		Name:                  "integration-template",
		Description:           "full reduction coupon",
		CouponType:            int8(domain.CouponTypeAmount),
		DiscountValue:         2000,
		ApplicableProductIDs:  "[]",
		ApplicableCategoryIDs: "[]",
		MinOrderAmount:        &minAmount,
		ValidType:             1,
		ValidStartTime:        &now,
		ValidEndTime:          ptrTime(now.Add(7 * 24 * time.Hour)),
		TotalCount:            100,
		IssuedCount:           0,
		PerUserLimit:          3,
		Status:                1,
	}

	err := database.Create(template).Error
	require.NoError(t, err)
	return template.ID
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
