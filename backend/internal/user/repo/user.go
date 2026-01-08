package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/cache"
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

// CachedUserRepository 使用了缓存的 repository 实现
type CachedUserRepository struct {
	dao   dao.UserDAO
	cache cache.UserCache
}

func NewUserRepository(d dao.UserDAO, c cache.UserCache) UserRepository {
	return &CachedUserRepository{
		dao: d,
		cache: c,
	}
}

func (r *CachedUserRepository) Create(ctx context.Context, u domain.User) (int64, error) {
	return r.dao.Insert(ctx, r.domainToEntity(u))
}

func (r *CachedUserRepository) Update(ctx context.Context, u domain.User) error {
	return r.dao.Update(ctx, r.domainToEntity(u))
}

func (r *CachedUserRepository) FindById(ctx context.Context, id int64) (domain.User, error) {
	user, err := r.dao.FindById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(user), nil
}

func (r *CachedUserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := r.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	return r.entityToDomain(user), nil
}

func (r *CachedUserRepository) Delete(ctx context.Context, id int64) error {
	return r.dao.Delete(ctx, id)
}

func (r *CachedUserRepository) domainToEntity(u domain.User) dao.User {
	entity := dao.User{
		UserName: u.UserName,
		Email:    u.Email,
		Password: u.Password,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
	}
	if u.ID > 0 {
		entity.ID = uint(u.ID)
	}
	return entity
}

func (r *CachedUserRepository) entityToDomain(u dao.User) domain.User {
	return domain.User{
		ID:       int64(u.ID),
		UserName: u.UserName,
		Email:    u.Email,
		Password: u.Password,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
	}
}
