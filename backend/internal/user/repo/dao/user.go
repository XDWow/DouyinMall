package dao

import (
	"context"
	"database/sql"
	"time"
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// ErrDataNotFound 通用的数据没找到
var ErrDataNotFound = gorm.ErrRecordNotFound
// 数据库错误：唯一索引冲突 转化为 业务错误
var ErrUserDuplicate = errors.New("用户邮箱或者手机号冲突")


type UserDAO interface {
	Insert(ctx context.Context, u User) (int64, error)
	Update(ctx context.Context, u User) error
	FindByPhone(ctx context.Context, phone string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindById(ctx context.Context, id int64) (User, error)
	Delete(ctx context.Context, id int64) error
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
		if me, ok := err.(*mysql.MySQLError); ok {
			const uniqueIndexErrNo uint16 = 1062
			if me.Number == uniqueIndexErrNo {
				return 0, ErrUserDuplicate
			}
		}
		return 0, err
	}
	return u.ID, nil
}

func (d *GORMUserDAO) Update(ctx context.Context, u User) error {
	// 使用 Updates 方法，只更新非零值字段
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"updated_at": time.Now(),
			"user_name": u.UserName,
			"email":     u.Email,
			"phone":     u.Phone,
			"avatar":    u.Avatar,
			"password":  u.Password,
		}).Error
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
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
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
	Email    sql.NullString `gorm:"uniqueIndex"`
	Password string         `gorm:"type:varchar(255)"`
	Phone    sql.NullString `gorm:"type:varchar(20);uniqueIndex"`
	Avatar   string
}
