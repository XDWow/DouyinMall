package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// ErrDataNotFound 閫氱敤鐨勬暟鎹病鎵惧埌
var ErrDataNotFound = gorm.ErrRecordNotFound

// 鏁版嵁搴撻敊璇細鍞竴绱㈠紩鍐茬獊 杞寲涓?涓氬姟閿欒
var ErrUserDuplicate = errors.New("鐢ㄦ埛閭鎴栬€呮墜鏈哄彿鍐茬獊")

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
	// 浣跨敤 Updates 鏂规硶锛屽彧鏇存柊闈為浂鍊煎瓧娈?
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"user_name":  u.UserName,
			"email":      u.Email,
			"phone":      u.Phone,
			"avatar":     u.Avatar,
			"password":   u.Password,
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

// 杞垹闄?
func (d *GORMUserDAO) Delete(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Model(&User{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

// 鎸佷箙鍖栨ā鍨?
type User struct {
	ID int64 `gorm:"primaryKey;autoIncrement"`
	// 杩欎袱涓椂闂村瓧娈碉紝gorm 鑳借瘑鍒殑锛屾洿鏂版彃鍏ユ搷浣滀細鑷姩鏇存柊杩欎袱瀛楁
	CreatedAt time.Time
	UpdatedAt time.Time
	// 杞垹闄ゅ瓧娈碉紝GORM 浼氳嚜鍔ㄥ鐞?gorm.DeletedAt 绫诲瀷
	// 鍗虫煡璇㈣嚜鍔ㄥ姞涓?WHERE deleted_at IS NULL
	DeletedAt gorm.DeletedAt `gorm:"index"`

	UserName string
	Email    sql.NullString `gorm:"type:varchar(191);uniqueIndex"`
	Password string         `gorm:"type:varchar(255)"`
	Phone    sql.NullString `gorm:"type:varchar(20);uniqueIndex"`
	Avatar   string
}


