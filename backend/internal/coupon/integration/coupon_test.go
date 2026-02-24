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
	couponRepo      domain.CouponRepository
	templateRepo    domain.CouponTemplateRepository
	operationRepo   domain.CouponOperationRepository

	// UseCase
	issueCouponUC         *usecase.IssueCouponUseCase
	listUserCouponsUC     *usecase.ListUserCouponsUseCase
	evaluateUC            *usecase.EvaluateOrderCouponsUseCase
	reserveUC             *usecase.ReserveCouponUseCase
	commitUC              *usecase.CommitCouponUseCase
	releaseUC             *usecase.ReleaseCouponUseCase
	refundUC              *usecase.RefundCouponUseCase

	// Job
	expireJob *job.ExpireCouponJob

	// 测试数据
	testUserID     int64
	testTemplateID int64
}

func TestCouponIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}
	suite.Run(t, new(CouponIntegrationSuite))
}

func (s *CouponIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 启动 MySQL 容器
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

	// 重试连接MySQL
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

	// 自动迁移表结构
	err = db.InitTables(gormDB)
	require.NoError(s.T(), err)

	// 初始化组件
	testLogger := logger.NewNopLogger()
	s.couponRepo = repository.NewCouponRepository(gormDB, testLogger)
	s.templateRepo = repository.NewCouponTemplateRepository(gormDB, testLogger)
	s.operationRepo = repository.NewCouponOperationRepository(gormDB, testLogger)

	// 初始化 UseCase
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

	// 初始化 Job
	s.expireJob = job.NewExpireCouponJob(s.couponRepo, testLogger)

	// 测试数据
	s.testUserID = 12345

	s.T().Log("✅ 测试环境初始化完成")
}

func (s *CouponIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(ctx)
	}
	s.T().Log("✅ 测试环境清理完成")
}

func (s *CouponIntegrationSuite) SetupTest() {
	// 每个测试前清理数据
	s.db.Exec("TRUNCATE TABLE coupons")
	s.db.Exec("TRUNCATE TABLE coupon_templates")
	s.db.Exec("TRUNCATE TABLE coupon_operations")

	// 创建测试优惠券模板
	s.createTestTemplate()
}

func (s *CouponIntegrationSuite) createTestTemplate() {
	now := time.Now()
	template := &db.CouponTemplate{
		Name:           "测试满减券",
		Description:    "满100减20",
		CouponType:     1, // 满减券
		DiscountValue:  2000,
		MinOrderAmount: ptrInt32(10000),
		ValidType:      1, // 固定时间
		ValidStartTime: &now,
		ValidEndTime:   ptrTime(now.Add(7 * 24 * time.Hour)),
		TotalCount:     100,
		IssuedCount:    0,
		PerUserLimit:   3,
		Status:         1, // 启用
	}

	err := s.db.Create(template).Error
	require.NoError(s.T(), err)
	s.testTemplateID = template.ID
}

// ==================== 测试用例 ====================

// TestIssueCoupon_Success 测试成功发放优惠券
func (s *CouponIntegrationSuite) TestIssueCoupon_Success() {
	ctx := context.Background()

	// 发放优惠券
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:user_12345_template_1_20260222",
	})

	require.NoError(s.T(), err)
	assert.NotZero(s.T(), output.CouponID)

	// 验证券已创建
	coupons, total, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(1), total)
	assert.Equal(s.T(), output.CouponID, coupons[0].ID)
	assert.Equal(s.T(), domain.UserCouponStatusUnused, coupons[0].Status)

	s.T().Log("✅ 发放优惠券测试通过")
}

// TestIssueCoupon_Idempotent 测试幂等发放
func (s *CouponIntegrationSuite) TestIssueCoupon_Idempotent() {
	ctx := context.Background()
	operationID := "coupon:test:user_12345_template_1_idempotent"

	// 第一次发放
	output1, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: operationID,
	})
	require.NoError(s.T(), err)

	// 第二次发放（相同operationID）
	output2, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: operationID,
	})
	require.NoError(s.T(), err)

	// 应该返回相同的券ID
	assert.Equal(s.T(), output1.CouponID, output2.CouponID)

	// 验证只有一张券
	_, total, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int32(1), total)

	s.T().Log("✅ 幂等发放测试通过")
}

// TestCouponLifecycle_ReserveAndCommit 测试预扣-确认流程
func (s *CouponIntegrationSuite) TestCouponLifecycle_ReserveAndCommit() {
	ctx := context.Background()
	orderID := int64(99999)

	// 1. 发放优惠券
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:lifecycle_1",
	})
	require.NoError(s.T(), err)
	couponID := output.CouponID

	// 2. 预扣优惠券（模拟下单）
	reserveOutput, err := s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{couponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)
	assert.True(s.T(), reserveOutput.Success)
	assert.Equal(s.T(), 1, reserveOutput.ReservedCount)

	// 验证券状态为Locked
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusLocked, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))
	assert.Equal(s.T(), orderID, coupons[0].OrderID)

	// 3. 确认核销（模拟支付成功）
	err = s.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 验证券状态为Used
	usedCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUsed, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(usedCoupons))

	s.T().Log("✅ 预扣-确认流程测试通过")
}

// TestCouponLifecycle_ReserveAndRelease 测试预扣-释放流程
func (s *CouponIntegrationSuite) TestCouponLifecycle_ReserveAndRelease() {
	ctx := context.Background()
	orderID := int64(88888)

	// 1. 发放优惠券
	output, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:lifecycle_2",
	})
	require.NoError(s.T(), err)
	couponID := output.CouponID

	// 2. 预扣优惠券
	_, err = s.reserveUC.Execute(ctx, usecase.ReserveCouponInput{
		UserID:    s.testUserID,
		CouponIDs: []int64{couponID},
		OrderID:   orderID,
	})
	require.NoError(s.T(), err)

	// 3. 释放优惠券（模拟订单取消）
	err = s.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 验证券状态恢复为Unused
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))
	assert.Equal(s.T(), int64(0), coupons[0].OrderID)

	s.T().Log("✅ 预扣-释放流程测试通过")
}

// TestCouponLifecycle_CommitAndRefund 测试确认-退还流程
func (s *CouponIntegrationSuite) TestCouponLifecycle_CommitAndRefund() {
	ctx := context.Background()
	orderID := int64(77777)

	// 1. 发放 -> 预扣 -> 确认
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

	// 2. 退还优惠券（模拟订单退款）
	err = s.refundUC.Execute(ctx, usecase.RefundCouponInput{
		OrderID: orderID,
	})
	require.NoError(s.T(), err)

	// 验证券状态恢复为Unused
	coupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusUnused, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(coupons))

	s.T().Log("✅ 确认-退还流程测试通过")
}

// TestEvaluateOrderCoupons 测试评估订单可用券
func (s *CouponIntegrationSuite) TestEvaluateOrderCoupons() {
	ctx := context.Background()

	// 1. 发放优惠券
	_, err := s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  s.testTemplateID,
		OperationID: "coupon:test:evaluate",
	})
	require.NoError(s.T(), err)

	// 2. 评估订单（满足门槛：100元）
	output, err := s.evaluateUC.Execute(ctx, usecase.EvaluateOrderCouponsInput{
		UserID: s.testUserID,
		Items: []domain.OrderItem{
			{ProductID: 1, CategoryID: 1, Subtotal: 10000}, // 100元
		},
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(output.Coupons))
	assert.True(s.T(), output.Coupons[0].Usable)

	// 3. 评估订单（不满足门槛：50元）
	output2, err := s.evaluateUC.Execute(ctx, usecase.EvaluateOrderCouponsInput{
		UserID: s.testUserID,
		Items: []domain.OrderItem{
			{ProductID: 1, CategoryID: 1, Subtotal: 5000}, // 50元
		},
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(output2.Coupons))
	assert.False(s.T(), output2.Coupons[0].Usable)
	assert.Contains(s.T(), output2.Coupons[0].Reason, "门槛")

	s.T().Log("✅ 评估订单可用券测试通过")
}

// TestExpireCouponJob 测试过期券扫描
func (s *CouponIntegrationSuite) TestExpireCouponJob() {
	ctx := context.Background()

	// 1. 创建已过期的模板
	pastTime := time.Now().Add(-24 * time.Hour)
	expiredTemplate := &db.CouponTemplate{
		Name:           "已过期券",
		CouponType:     1,
		DiscountValue:  1000,
		ValidType:      1,
		ValidStartTime: ptrTime(pastTime.Add(-7 * 24 * time.Hour)),
		ValidEndTime:   &pastTime, // 昨天过期
		TotalCount:     10,
		Status:         1,
		PerUserLimit:   1,
	}
	err := s.db.Create(expiredTemplate).Error
	require.NoError(s.T(), err)

	// 2. 发放过期券
	_, err = s.issueCouponUC.Execute(ctx, usecase.IssueCouponInput{
		UserID:      s.testUserID,
		TemplateID:  expiredTemplate.ID,
		OperationID: "coupon:test:expired",
	})
	require.NoError(s.T(), err)

	// 3. 执行过期扫描Job
	err = s.expireJob.Run()
	require.NoError(s.T(), err)

	// 4. 验证券状态已更新为Expired
	expiredCoupons, _, err := s.couponRepo.ListByUserID(ctx, s.testUserID, domain.UserCouponStatusExpired, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(expiredCoupons))

	s.T().Log("✅ 过期券扫描测试通过")
}

// ==================== 辅助函数 ====================

func ptrInt32(v int32) *int32 {
	return &v
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
