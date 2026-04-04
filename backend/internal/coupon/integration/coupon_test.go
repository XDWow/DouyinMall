//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/job"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type CouponIntegrationSuite struct {
	suite.Suite
	mysqlContainer testcontainers.Container
	db             *gorm.DB

	// Repository
	couponRepo    domain.CouponRepository
	templateRepo  domain.CouponTemplateRepository
	operationRepo domain.CouponOperationRepository

	// UseCase
	issueCouponUC     *usecase.IssueCouponUseCase
	listUserCouponsUC *usecase.ListUserCouponsUseCase
	evaluateUC        *usecase.EvaluateOrderCouponsUseCase
	reserveUC         *usecase.ReserveCouponUseCase
	commitUC          *usecase.CommitCouponUseCase
	releaseUC         *usecase.ReleaseCouponUseCase
	refundUC          *usecase.RefundCouponUseCase

	// Job
	expireJob *job.ExpireCouponJob

	// 娴嬭瘯鏁版嵁
	testUserID     int64
	testTemplateID int64
}

func TestCouponIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("璺宠繃闆嗘垚娴嬭瘯锛堢煭妯″紡锛?)
	}
	suite.Run(t, new(CouponIntegrationSuite))
}

func (s *CouponIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 鍚姩 MySQL 瀹瑰櫒
	mysqlReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "coupon_test",
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

	dsn := "root:root@tcp(" + mysqlHost + ":" + mysqlPort.Port() + ")/coupon_test?charset=utf8mb4&parseTime=True&loc=Local"

	// 閲嶈瘯杩炴帴MySQL
	var gormDB *gorm.DB
	for i := 0; i < 10; i++ {
		gormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, err := gormDB.DB()
			if err == nil && sqlDB.Ping() == nil {
				break
			}
		}
		if i < 9 {
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(s.T(), err)
	s.db = gormDB

	// 鑷姩杩佺Щ琛ㄧ粨鏋?
	err = db.InitTables(gormDB)
	require.NoError(s.T(), err)

	// 鍒濆鍖栫粍浠?
	testLogger := logger.NewNopLogger()
	s.couponRepo = repository.NewCouponRepository(gormDB, testLogger)
	s.templateRepo = repository.NewCouponTemplateRepository(gormDB, testLogger)
	s.operationRepo = repository.NewCouponOperationRepository(gormDB, testLogger)

	// 鍒濆鍖?UseCase
	s.issueCouponUC = usecase.NewIssueCouponUseCase(
		s.templateRepo,
		s.couponRepo,
		s.operationRepo,
	)
	s.listUserCouponsUC = usecase.NewListUserCouponsUseCase(s.couponRepo)
	s.evaluateUC = usecase.NewEvaluateOrderCouponsUseCase(s.couponRepo)
	s.reserveUC = usecase.NewReserveCouponUseCase(s.couponRepo)
	s.commitUC = usecase.NewCommitCouponUseCase(s.couponRepo)
	s.releaseUC = usecase.NewReleaseCouponUseCase(s.couponRepo)
	s.refundUC = usecase.NewRefundCouponUseCase(s.couponRepo)

	// 鍒濆鍖?Job
	s.expireJob = job.NewExpireCouponJob(s.couponRepo, testLogger)

	// 娴嬭瘯鏁版嵁
	s.testUserID = 12345

	s.T().Log("鉁?娴嬭瘯鐜鍒濆鍖栧畬鎴?)
}

func (s *CouponIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(ctx)
	}
	s.T().Log("鉁?娴嬭瘯鐜娓呯悊瀹屾垚")
}

func (s *CouponIntegrationSuite) SetupTest() {
	// 姣忎釜娴嬭瘯鍓嶆竻鐞嗘暟鎹?
	s.db.Exec("TRUNCATE TABLE coupons")
	s.db.Exec("TRUNCATE TABLE coupon_templates")
	s.db.Exec("TRUNCATE TABLE coupon_operations")

	// 鍒涘缓娴嬭瘯浼樻儬鍒告ā鏉?
	s.createTestTemplate()
}

func (s *CouponIntegrationSuite) createTestTemplate() {
	now := time.Now()
	template := &db.CouponTemplate{
		Name:           "娴嬭瘯婊″噺鍒?,
		Description:    "婊?00鍑?0",
		CouponType:     1, // 婊″噺鍒?
		DiscountValue:  2000,
		MinOrderAmount: ptrInt32(10000),
		ValidType:      1, // 鍥哄畾鏃堕棿
		ValidStartTime: &now,
		ValidEndTime:   ptrTime(now.Add(7 * 24 * time.Hour)),
		TotalCount:     100,
		IssuedCount:    0,
		PerUserLimit:   3,
		Status:         1, // 鍚敤
	}

	err := s.db.Create(template).Error
	require.NoError(s.T(), err)
	s.testTemplateID = template.ID
}

// ==================== 娴嬭瘯鐢ㄤ緥 ====================

// TestIssueCoupon_Success 娴嬭瘯鎴愬姛鍙戞斁浼樻儬鍒?
func (s *CouponIntegrationSuite) TestIssueCoupon_Success() {
	ctx := context.Background()

	// 鍙戞斁浼樻儬鍒?
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:user_12345_template_1_20260222",
	})

	require.NoError(s.T(), err)
	assert.NotZero(s.T(), output.CouponID)

	// 楠岃瘉鍒稿凡鍒涘缓
	coupons, total, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(1), total)
	assert.Equal(s.T(), output.CouponID, coupons[0].ID)
	assert.Equal(s.T(), domain.UserCouponStatusUnused, coupons[0].Status)

	s.T().Log("鉁?鍙戞斁浼樻儬鍒告祴璇曢€氳繃")
}

// TestIssueCoupon_Idempotent 娴嬭瘯骞傜瓑鍙戞斁
func (s *CouponIntegrationSuite) TestIssueCoupon_Idempotent() {
	ctx := context.Background()
	operationID := "coupon:test:user_12345_template_1_idempotent"

	// 绗竴娆″彂鏀?
	output1, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: operationID,
	})
	require.NoError(s.T(), err)

	// 绗簩娆″彂鏀撅紙鐩稿悓operationID锛?
	output2, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: operationID,
	})
	require.NoError(s.T(), err)

	// 搴旇杩斿洖鐩稿悓鐨勫埜ID
	assert.Equal(s.T(), output1.CouponID, output2.CouponID)

	// 楠岃瘉鍙湁涓€寮犲埜
	_, total, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(1), total)

	s.T().Log("鉁?骞傜瓑鍙戞斁娴嬭瘯閫氳繃")
}

// TestCouponLifecycle_ReserveAndCommit 娴嬭瘯棰勬墸-纭娴佺▼
func (s *CouponIntegrationSuite) TestCouponLifecycle_ReserveAndCommit() {
	ctx := context.Background()
	orderID := int64(99999)

	// 1. 鍙戞斁浼樻儬鍒?
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:lifecycle_1",
	})
	require.NoError(s.T(), err)
	couponID := output.CouponID

	// 2. 棰勬墸浼樻儬鍒革紙妯℃嫙涓嬪崟锛?
	reserveOutput, err := s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{couponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)
	assert.True(s.T(), reserveOutput.Success)
	assert.Equal(s.T(), 1, reserveOutput.ReservedCount)

	// 楠岃瘉鍒哥姸鎬佷负Locked
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusLocked, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))
	assert.Equal(s.T(), orderID, coupons[0].OrderID)

	// 3. 纭鏍搁攢锛堟ā鎷熸敮浠樻垚鍔燂級
	err = s.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 楠岃瘉鍒哥姸鎬佷负Used
	usedCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUsed, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(usedCoupons))

	s.T().Log("鉁?棰勬墸-纭娴佺▼娴嬭瘯閫氳繃")
}

// TestCouponLifecycle_ReserveAndRelease 娴嬭瘯棰勬墸-閲婃斁娴佺▼
func (s *CouponIntegrationSuite) TestCouponLifecycle_ReserveAndRelease() {
	ctx := context.Background()
	orderID := int64(88888)

	// 1. 鍙戞斁浼樻儬鍒?
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:lifecycle_2",
	})
	require.NoError(s.T(), err)
	couponID := output.CouponID

	// 2. 棰勬墸浼樻儬鍒?
	_, err = s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{couponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)

	// 3. 閲婃斁浼樻儬鍒革紙妯℃嫙璁㈠崟鍙栨秷锛?
	err = s.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 楠岃瘉鍒哥姸鎬佹仮澶嶄负Unused
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))
	assert.Equal(s.T(), int64(0), coupons[0].OrderID)
	assert.True(s.T(), coupons[0].UsedAt.IsZero())

	s.T().Log("鉁?棰勬墸-閲婃斁娴佺▼娴嬭瘯閫氳繃")
}

// TestCouponLifecycle_CommitAndRefund 娴嬭瘯纭-閫€杩樻祦绋?
func (s *CouponIntegrationSuite) TestCouponLifecycle_CommitAndRefund() {
	ctx := context.Background()
	orderID := int64(77777)

	// 1. 鍙戞斁 -> 棰勬墸 -> 纭
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:lifecycle_3",
	})
	require.NoError(s.T(), err)

	_, err = s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{output.CouponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)

	err = s.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 2. 閫€杩樹紭鎯犲埜锛堟ā鎷熻鍗曢€€娆撅級
	err = s.refundUC.Execute(ctx, usecase.RefundCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 楠岃瘉鍒哥姸鎬佹仮澶嶄负Unused
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))
	assert.Equal(s.T(), int64(0), coupons[0].OrderID)
	assert.True(s.T(), coupons[0].UsedAt.IsZero())

	s.T().Log("鉁?纭-閫€杩樻祦绋嬫祴璇曢€氳繃")
}

// TestEvaluateOrderCoupons 娴嬭瘯璇勪及璁㈠崟鍙敤鍒?
func (s *CouponIntegrationSuite) TestCouponLifecycle_ReserveAllOrNothing() {
	ctx := context.Background()
	orderID := int64(66666)

	output1, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:reserve_all_or_nothing_1",
	})
	require.NoError(s.T(), err)

	output2, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:reserve_all_or_nothing_2",
	})
	require.NoError(s.T(), err)

	_, err = s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{output1.CouponID},
		OrderID:   55555,
	})
	require.NoError(s.T(), err)

	reserveOutput, err := s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{output1.CouponID, output2.CouponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)
	assert.False(s.T(), reserveOutput.Success)
	assert.Len(s.T(), reserveOutput.Failures, 1)
	assert.Equal(s.T(), output1.CouponID, reserveOutput.Failures[0].CouponID)

	lockedCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusLocked, 1, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), lockedCoupons, 1)
	assert.Equal(s.T(), int64(55555), lockedCoupons[0].OrderID)

	unusedCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), unusedCoupons, 1)
	assert.Equal(s.T(), output2.CouponID, unusedCoupons[0].ID)
}

func (s *CouponIntegrationSuite) TestEvaluateOrderCoupons() {
	ctx := context.Background()

	// 1. 鍙戞斁浼樻儬鍒?
	_, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:evaluate",
	})
	require.NoError(s.T(), err)

	// 2. 璇勪及璁㈠崟锛堟弧瓒抽棬妲涳細100鍏冿級
	output, err := s.evaluateUC.Execute(ctx, usecase.EvaluateOrderCouponsInput{
		UserID: s.testUserID,
		Items: []domain.OrderItem{
			{ProductID: 1, CategoryID: 1, Subtotal: 10000}, // 100鍏?
		},
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(output.Coupons))
	assert.True(s.T(), output.Coupons[0].Usable)

	// 3. 璇勪及璁㈠崟锛堜笉婊¤冻闂ㄦ锛?0鍏冿級
	output2, err := s.evaluateUC.Execute(ctx, usecase.EvaluateOrderCouponsInput{
		UserID: s.testUserID,
		Items: []domain.OrderItem{
			{ProductID: 1, CategoryID: 1, Subtotal: 5000}, // 50鍏?
		},
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(output2.Coupons))
	assert.False(s.T(), output2.Coupons[0].Usable)
	assert.Contains(s.T(), output2.Coupons[0].Reason, "闂ㄦ")

	s.T().Log("鉁?璇勪及璁㈠崟鍙敤鍒告祴璇曢€氳繃")
}

// TestExpireCouponJob 娴嬭瘯杩囨湡鍒告壂鎻?
func (s *CouponIntegrationSuite) TestExpireCouponJob() {
	ctx := context.Background()

	// 1. 鍒涘缓宸茶繃鏈熺殑妯℃澘
	pastTime := time.Now().Add(-24 * time.Hour)
	expiredTemplate := &db.CouponTemplate{
		Name:           "宸茶繃鏈熷埜",
		CouponType:     1,
		DiscountValue:  1000,
		ValidType:      1,
		ValidStartTime: ptrTime(pastTime.Add(-7 * 24 * time.Hour)),
		ValidEndTime:   &pastTime, // 鏄ㄥぉ杩囨湡
		TotalCount:     10,
		Status:         1,
		PerUserLimit:   1,
	}
	err := s.db.Create(expiredTemplate).Error
	require.NoError(s.T(), err)

	// 2. 鍙戞斁杩囨湡鍒?
	_, err = s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  expiredTemplate.ID,
		OperationID: "coupon:test:expired",
	})
	require.NoError(s.T(), err)

	// 3. 鎵ц杩囨湡鎵弿Job
	err = s.expireJob.Run()
	require.NoError(s.T(), err)

	// 4. 楠岃瘉鍒哥姸鎬佸凡鏇存柊涓篍xpired
	expiredCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusExpired, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(expiredCoupons))

	s.T().Log("鉁?杩囨湡鍒告壂鎻忔祴璇曢€氳繃")
}

// ==================== 杈呭姪鍑芥暟 ====================

func ptrInt32(v int32) *int32 {
	return &v
}

func ptrTime(t time.Time) *time.Time {
	return &t
}


