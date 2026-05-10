package dao

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=product.go -destination=mocks/product_dao_mock.go -package=mocks

const defaultCurrency = "CNY"

var ErrDataNotFound = gorm.ErrRecordNotFound

type ProductDao interface {
	ListProducts(ctx context.Context, page, pageSize int64, category string) (products []Product, err error)
	FindByID(ctx context.Context, id int64) (product Product, err error)
	FindPriceInStock(ctx context.Context, productID, skuID int64) (price int64, currency string, inStock bool, err error)
	Insert(ctx context.Context, product Product) (id int64, err error)
	Update(ctx context.Context, product Product) (err error)
	Delete(ctx context.Context, id, userID int64) (err error)
	UpsertSKU(ctx context.Context, sku ProductSKU) error
	DeleteSKUsByProductID(ctx context.Context, productID int64) error
}

type GORMProductDao struct {
	db *gorm.DB
}

func NewGORMProductDao(db *gorm.DB) ProductDao {
	return &GORMProductDao{db: db}
}

func (d *GORMProductDao) ListProducts(ctx context.Context, page, pageSize int64, category string) (products []Product, err error) {
	query := d.db.WithContext(ctx).Model(&Product{}).Where("in_stock = ?", true)

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

func (d *GORMProductDao) FindPriceInStock(ctx context.Context, productID, skuID int64) (price int64, currency string, inStock bool, err error) {
	if skuID > 0 {
		var sku ProductSKU
		err = d.db.WithContext(ctx).
			Where("product_id = ? AND sku_id = ?", productID, skuID).
			First(&sku).Error
		return sku.Price, normalizeCurrency(sku.Currency), sku.InStock, err
	}

	var result struct {
		Price    int64
		Currency string
		InStock  bool
	}
	err = d.db.WithContext(ctx).
		Model(&Product{}).
		Select("price, currency, in_stock").
		Where("id = ?", productID).
		First(&result).Error
	return result.Price, normalizeCurrency(result.Currency), result.InStock, err
}

func (d *GORMProductDao) Insert(ctx context.Context, product Product) (id int64, err error) {
	product.Currency = normalizeCurrency(product.Currency)
	err = d.db.WithContext(ctx).Create(&product).Error
	if err != nil {
		return 0, err
	}
	return product.ID, nil
}

func (d *GORMProductDao) Update(ctx context.Context, product Product) (err error) {
	product.Currency = normalizeCurrency(product.Currency)
	return d.db.WithContext(ctx).
		Model(&Product{}).
		Where("id = ?", product.ID).
		Updates(map[string]interface{}{
			"name":          product.Name,
			"price":         product.Price,
			"currency":      product.Currency,
			"in_stock":      product.InStock,
			"merchant_id":   product.MerchantID,
			"merchant_name": product.MerchantName,
			"description":   product.Description,
			"picture":       product.Picture,
			"slide_imgs":    product.SlideImgs,
			"categories":    product.Categories,
		}).Error
}

func (d *GORMProductDao) Delete(ctx context.Context, id, userID int64) (err error) {
	return d.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", id, userID).Delete(&Product{}).Error
}

func (d *GORMProductDao) UpsertSKU(ctx context.Context, sku ProductSKU) error {
	if sku.ProductID <= 0 || sku.SKUID <= 0 {
		return nil
	}
	sku.Currency = normalizeCurrency(sku.Currency)
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "product_id"},
				{Name: "sku_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"price", "currency", "in_stock", "updated_at"}),
		}).
		Create(&sku).Error
}

func (d *GORMProductDao) DeleteSKUsByProductID(ctx context.Context, productID int64) error {
	return d.db.WithContext(ctx).Where("product_id = ?", productID).Delete(&ProductSKU{}).Error
}

type Product struct {
	ID int64 `gorm:"primaryKey;autoIncrement;comment:Product ID"`

	CreatedAt time.Time      `gorm:"autoCreateTime;comment:Created time"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;comment:Updated time"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:Deleted time"`

	Name       string `gorm:"type:varchar(255);not null;index;comment:Product name"`
	Price      int64  `gorm:"type:bigint;not null;default:0;comment:Default price in minor units"`
	Currency   string `gorm:"type:varchar(16);not null;default:CNY;comment:Default currency code"`
	InStock    bool   `gorm:"type:boolean;not null;default:true;comment:Default availability"`
	MerchantID int64  `gorm:"type:bigint;not null;index;comment:Merchant ID"`

	Description  sql.NullString `gorm:"type:text;comment:Product description"`
	Picture      sql.NullString `gorm:"type:varchar(512);comment:Cover image URL"`
	SlideImgs    string         `gorm:"type:json;comment:Gallery image URLs as JSON"`
	Categories   string         `gorm:"type:json;comment:Category labels as JSON"`
	MerchantName sql.NullString `gorm:"type:varchar(255);comment:Merchant name"`
}

func (Product) TableName() string {
	return "product"
}

type ProductSKU struct {
	ID int64 `gorm:"primaryKey;autoIncrement;comment:Row ID"`

	ProductID int64  `gorm:"type:bigint;not null;uniqueIndex:uidx_product_sku;index;comment:Product ID"`
	SKUID     int64  `gorm:"column:sku_id;type:bigint;not null;uniqueIndex:uidx_product_sku;index;comment:SKU ID"`
	Price     int64  `gorm:"type:bigint;not null;default:0;comment:SKU price in minor units"`
	Currency  string `gorm:"type:varchar(16);not null;default:CNY;comment:SKU currency code"`
	InStock   bool   `gorm:"type:boolean;not null;default:true;comment:SKU availability"`

	CreatedAt time.Time      `gorm:"autoCreateTime;comment:Created time"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;comment:Updated time"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:Deleted time"`
}

func (ProductSKU) TableName() string {
	return "product_sku"
}

func normalizeCurrency(currency string) string {
	if currency == "" {
		return defaultCurrency
	}
	return currency
}
