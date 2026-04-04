package dao

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

//go:generate mockgen -source=product.go -destination=mocks/product_dao_mock.go -package=mocks

var ErrDataNotFound = gorm.ErrRecordNotFound

type ProductDao interface {
	ListProducts(ctx context.Context, page, pageSize int64, category string) (products []Product, err error)
	FindByID(ctx context.Context, id int64) (product Product, err error)
	FindPriceInStock(ctx context.Context, id int64) (price int64, inStock bool, err error)
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

	// 鍙湅鏈夎揣
	query = query.Where("in_stock = ?", true)

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

func (d *GORMProductDao) FindPriceInStock(ctx context.Context, id int64) (price int64, inStock bool, err error) {
	var result struct {
		Price   int64
		InStock bool
	}
	err = d.db.WithContext(ctx).
		Model(&Product{}).
		Select("price, in_stock").
		Where("id = ?", id).
		First(&result).Error
	return result.Price, result.InStock, err
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
	ID int64 `gorm:"primaryKey;autoIncrement;comment:鍟嗗搧ID"`

	// GORM 鑷姩绠＄悊鐨勬椂闂村瓧娈?
	CreatedAt time.Time      `gorm:"autoCreateTime;comment:鍒涘缓鏃堕棿"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;comment:鏇存柊鏃堕棿"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:鍒犻櫎鏃堕棿(杞垹闄?"`

	Name       string `gorm:"type:varchar(255);not null;index;comment:鍟嗗搧鍚嶇О"`
	Price      int64  `gorm:"type:bigint;not null;default:0;comment:鍟嗗搧浠锋牸(鍒?"`
	InStock    bool   `gorm:"type:boolean;not null;default:true;comment:鏄惁鏈夎揣(鏉ヨ嚜搴撳瓨鏈嶅姟浜嬩欢)"`
	MerchantID int64  `gorm:"type:bigint;not null;index;comment:鍟嗗ID"`

	Description  sql.NullString `gorm:"type:text;comment:鍟嗗搧鎻忚堪"`
	Picture      sql.NullString `gorm:"type:varchar(512);comment:鍟嗗搧涓诲浘URL"`
	SlideImgs    string         `gorm:"type:json;comment:鍟嗗搧杞挱鍥綰RL鍒楄〃(JSON鏁扮粍)"`
	Categories   string         `gorm:"type:json;comment:鍟嗗搧鍒嗙被鍚嶇О鍒楄〃(JSON鏁扮粍)"`
	MerchantName sql.NullString `gorm:"type:varchar(255);comment:鍟嗗鍚嶇О(鍐椾綑瀛楁)"`

	// ========== 鎵╁睍瀛楁锛堝彲閫夛級==========
	// Status 鍟嗗搧鐘舵€侊細涓婃灦/涓嬫灦/瀹℃牳涓瓑
	// Status int8 `gorm:"type:tinyint;not null;default:1;comment:鍟嗗搧鐘舵€?1:涓婃灦 2:涓嬫灦 3:瀹℃牳涓?"`

	// Sort 鎺掑簭鏉冮噸锛氱敤浜庡晢鍝佸垪琛ㄦ帓搴?
	// Sort int64 `gorm:"type:bigint;not null;default:0;comment:鎺掑簭鏉冮噸"`
}

func (Product) TableName() string {
	return "product"
}


