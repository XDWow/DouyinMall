package dao

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

var ErrDataNotFound = gorm.ErrRecordNotFound

type ProductDao interface {
	ListProducts(ctx context.Context, page, pageSize int64, category string) (products []Product, err error)
	FindByID(ctx context.Context, id int64) (product Product, err error)
	FindPriceStock(ctx context.Context, id int64) (price, stock int64, err error)
	Insert(ctx context.Context, product Product) (id int64, err error)
	Update(ctx context.Context, product Product) (err error)
	Delete(ctx context.Context, id, userID int64) (err error)
}

type GORMProductDao struct {
	db *gorm.DB
}

func NewGORMProductDao(db *gorm.DB) ProductDao {
	return &GORMProductDao{
		db: db,
	}
}

func (d *GORMProductDao) ListProducts(ctx context.Context, page, pageSize int64, category string) (products []Product, err error) {
	query := d.db.WithContext(ctx).Model(&Product{})

	if category != "" {
		query = query.Where("JSON_CONTAINS(categories, ?)", `"`+category+`"`)
	}

	err = query.
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&products).Error
	return products, err
}

func (d *GORMProductDao) FindByID(ctx context.Context, id int64) (product Product, err error) {
	err = d.db.WithContext(ctx).Where("id = ?", id).First(&product).Error
	return product, err
}

func (d *GORMProductDao) FindPriceStock(ctx context.Context, id int64) (price, stock int64, err error) {
	var result struct {
		Price int64
		Stock int64
	}
	err = d.db.WithContext(ctx).
		Model(&Product{}).
		Select("price, stock").
		Where("id = ?", id).
		First(&result).Error
	return result.Price, result.Stock, err
}

func (d *GORMProductDao) Insert(ctx context.Context, product Product) (id int64, err error) {
	err = d.db.WithContext(ctx).Create(&product).Error
	if err != nil {
		return 0, err
	}
	return product.ID, nil
}

func (d *GORMProductDao) Update(ctx context.Context, product Product) (err error) {
	return d.db.WithContext(ctx).
		Model(&Product{}).
		Where("id = ?", product.ID).
		Updates(&product).Error
}

func (d *GORMProductDao) Delete(ctx context.Context, id, userID int64) (err error) {
	return d.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", id, userID).Delete(&Product{}).Error
}

type Product struct {
	ID int64 `gorm:"primaryKey;autoIncrement;comment:商品ID"`

	// GORM 自动管理的时间字段
	CreatedAt time.Time      `gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;comment:更新时间"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间(软删除)"`

	Name       string `gorm:"type:varchar(255);not null;index;comment:商品名称"`
	Price      int64  `gorm:"type:bigint;not null;default:0;comment:商品价格(分)"`
	Stock      int64  `gorm:"type:bigint;not null;default:0;comment:商品库存"`
	MerchantID int64  `gorm:"type:bigint;not null;index;comment:商家ID"`

	Description  sql.NullString `gorm:"type:text;comment:商品描述"`
	Picture      sql.NullString `gorm:"type:varchar(512);comment:商品主图URL"`
	SlideImgs    string         `gorm:"type:json;comment:商品轮播图URL列表(JSON数组)"`
	Categories   string         `gorm:"type:json;comment:商品分类名称列表(JSON数组)"`
	MerchantName sql.NullString `gorm:"type:varchar(255);comment:商家名称(冗余字段)"`

	// ========== 扩展字段（可选）==========
	// Status 商品状态：上架/下架/审核中等
	// Status int8 `gorm:"type:tinyint;not null;default:1;comment:商品状态(1:上架 2:下架 3:审核中)"`

	// Sort 排序权重：用于商品列表排序
	// Sort int64 `gorm:"type:bigint;not null;default:0;comment:排序权重"`
}

func (Product) TableName() string {
	return "product"
}
