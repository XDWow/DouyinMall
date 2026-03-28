package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

type paymentRepository struct {
	db *gorm.DB
	l  logger.LoggerV1
}

func NewPaymentRepository(db *gorm.DB, l logger.LoggerV1) domain.PaymentRepository {
	return &paymentRepository{
		db: db,
		l:  l,
	}
}

func (repo *paymentRepository) AddPayment(ctx context.Context, pmt domain.Payment) error {
	dbPmt := toDBPayment(pmt)
	return repo.db.WithContext(ctx).Create(&dbPmt).Error
}

func (repo *paymentRepository) UpdatePayment(ctx context.Context, pmt domain.Payment) error {
	// Updates(&struct) 会自动忽略零值，只更新有值的字段
	dbPmt := toDBPayment(pmt)
	return repo.db.WithContext(ctx).
		Model(&db.Payment{}).
		Where("biz_trade_no = ?", pmt.BizTradeNo).
		Updates(&dbPmt).Error
}

func (repo *paymentRepository) GetPayment(ctx context.Context, bizTradeNo string) (domain.Payment, error) {
	var pmtModel db.Payment
	err := repo.db.WithContext(ctx).
		Where("biz_trade_no = ?", bizTradeNo).
		First(&pmtModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Payment{}, domain.ErrPaymentNotFound
		}
		return domain.Payment{}, err
	}
	return toDomainPayment(pmtModel), nil
}

func (repo *paymentRepository) FindExpiredPayment(ctx context.Context,
	limit int, t time.Time) ([]domain.Payment, error) {
	models := make([]db.Payment, 0)
	query := repo.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", domain.PaymentStatusInit.AsUint8(), t).
		Order("updated_at ASC, id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&models).Error
	if err != nil {
		return nil, err
	}
	res := make([]domain.Payment, len(models))
	for i, model := range models {
		res[i] = toDomainPayment(model)
	}
	return res, nil
}

func toDomainPayment(pmt db.Payment) domain.Payment {
	return domain.Payment{
		ID:          pmt.ID,
		BizTradeNo:  pmt.BizTradeNO,
		Description: pmt.Description,
		Amt: domain.Amount{
			Currency: pmt.Currency,
			Total:    pmt.Amt,
		},
		Status: domain.PaymentStatus(pmt.Status),
		TxnID:  pmt.TxnID.String, // sql.NullString → string
	}
}

func toDBPayment(pmt domain.Payment) db.Payment {
	dbPmt := db.Payment{
		BizTradeNO:  pmt.BizTradeNo,
		Description: pmt.Description,
		Currency:    pmt.Amt.Currency,
		Amt:         pmt.Amt.Total,
		Status:      pmt.Status.AsUint8(),
	}
	if pmt.TxnID != "" {
		dbPmt.TxnID = sql.NullString{String: pmt.TxnID, Valid: true}
	}
	return dbPmt
}
