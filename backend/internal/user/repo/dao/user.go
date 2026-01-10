package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// ErrDataNotFound 通用的数据没找到
var ErrDataNotFound = gorm.ErrRecordNotFound

type UserDAO interface {
	Insert(ctx context.Context, u User) error
	UpdateNonZeroFields(ctx context.Context, u User) error
	FindByPhone(ctx context.Context, phone string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindById(ctx context.Context, id int64) (User, error)
}

type GORMUserDAO struct {
	db *gorm.DB
}

func NewGORMUserDAO(db *gorm.DB) UserDAO {
	return &GORMUserDAO{
		db: db,
	}
}

func (d *GORMUserDAO) Insert(ctx context.Context, u User) (int64, error) {
	err := d.db.WithContext(ctx).Create(&u).Error
	if err != nil {
		return 0, err
	}
	return int64(u.ID), nil
}

func (d *GORMUserDAO) UpdateNonZeroFields(ctx context.Context, u User) error {
	// 使用 Updates 方法，只更新非零值字段
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"user_name": u.UserName,
			"avatar":    u.Avatar,
			// 如果需要更新敏感字段，可以在这里添加
		}).Error
}

func (d *GORMUserDAO) UpdateWithMap(ctx context.Context, id uint, updates map[string]interface{}) error {
	return d.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(updates).Error
}

func (d *GORMUserDAO) FindById(ctx context.Context, id int64) (User, error) {
	var user User
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	return user, err
}

func (d *GORMUserDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := d.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return user, err
}

func (d *GORMUserDAO) FindByPhone(ctx context.Context, phone string) (User, error) {
	var user User
	err := d.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	return user, err
}

// 软删除
func (d *GORMUserDAO) Delete(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Delete(&User{}, id).Error
}

// 持久化模型
type User struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time
	UpdatedAt time.Time
	// 软删除字段，GORM 会自动处理 gorm.DeletedAt 类型
	// 即查询自动加上 WHERE deleted_at IS NULL
	DeletedAt gorm.DeletedAt `gorm:"index"`

	UserName string
	Email    string `gorm:"uniqueIndex"`
	Password string `gorm:"type:varchar(255);not null"`
	Phone    string `gorm:"type:varchar(20);not null"`
	Avatar   string
}