package dao

import (
	"context"

	"gorm.io/gorm"
)

// ErrDataNotFound 通用的数据没找到
var ErrDataNotFound = gorm.ErrRecordNotFound

type User struct {
	// 组合 gorm 的基础模型
	gorm.Model
	UserName string
	Email    string `gorm:"unique;unique_index"`
	Password string `gorm:"type:varchar(255) not null"`
	Phone    string `gorm:"type:varchar(20) not null"`
	Avatar   string
}

// UserDAO 用户数据访问对象
type UserDAO struct {
	db *gorm.DB
}

// NewUserDAO 创建 UserDAO 实例
func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{
		db: db,
	}
}

// Insert 插入用户记录
func (d *UserDAO) Insert(ctx context.Context, u User) (int64, error) {
	err := d.db.WithContext(ctx).Create(&u).Error
	if err != nil {
		return 0, err
	}
	return int64(u.ID), nil
}

// Update 更新用户记录，只更新非零值字段
func (d *UserDAO) Update(ctx context.Context, u User) error {
	// 使用 Updates 方法，只更新非零值字段
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"user_name": u.UserName,
			"avatar":    u.Avatar,
			// 如果需要更新敏感字段，可以在这里添加
		}).Error
}

// UpdateWithMap 使用 map 更新指定字段
func (d *UserDAO) UpdateWithMap(ctx context.Context, id uint, updates map[string]interface{}) error {
	return d.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(updates).Error
}

// FindById 根据 ID 查找用户
func (d *UserDAO) FindById(ctx context.Context, id int64) (User, error) {
	var user User
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	return user, err
}

// FindByEmail 根据邮箱查找用户
func (d *UserDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := d.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return user, err
}

// Delete 删除用户（软删除）
func (d *UserDAO) Delete(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Delete(&User{}, id).Error
}
