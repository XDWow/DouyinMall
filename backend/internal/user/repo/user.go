package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
)

var ErrUserNorFound = dao.ErrDataNotFound

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (int64, error)
	// Update 更新数据，只有非 0 值才会更新
	Update(ctx context.Context, u domain.User) error
	FindById(ctx context.Context, id int64) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	Delete(ctx context.Context, id int64) error
}

// userRepository 实现 UserRepository 接口
type userRepository struct {
	dao *dao.UserDAO
}

// NewUserRepository 创建 UserRepository 实例
func NewUserRepository(d *dao.UserDAO) UserRepository {
	return &userRepository{
		dao: d,
	}
}

func (r *userRepository) Create(ctx context.Context, u domain.User) (int64, error) {
	return r.dao.Insert(ctx, r.domainToEntity(u))
}

func (r *userRepository) Update(ctx context.Context, u domain.User) error {
	return r.dao.Update(ctx, r.domainToEntity(u))
}

func (r *userRepository) FindById(ctx context.Context, id int64) (domain.User, error) {
	user, err := r.dao.FindById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(user), nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := r.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(user), nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	return r.dao.Delete(ctx, id)
}

// domainToEntity 将领域模型转换为数据库实体
func (r *userRepository) domainToEntity(u domain.User) dao.User {
	entity := dao.User{
		UserName: u.UserName,
		Email:    u.Email,
		Password: u.Password,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
	}
	// 如果有 ID，设置到 Model 中（用于更新操作）
	if u.ID > 0 {
		entity.ID = uint(u.ID)
	}
	return entity
}

// entityToDomain 将数据库实体转换为领域模型
func (r *userRepository) entityToDomain(u dao.User) domain.User {
	return domain.User{
		ID:       int64(u.ID),
		UserName: u.UserName,
		Email:    u.Email,
		Password: u.Password,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
	}
}
