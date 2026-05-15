package repo

import (
	"context"
	"database/sql"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
)

var ErrUserDuplicateEmail = dao.ErrUserDuplicate
var ErrUserNorFound = dao.ErrDataNotFound

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (int64, error)
	// Update 更新数据，只有非 0 值才会更新。
	Update(ctx context.Context, u domain.User) error
	FindById(ctx context.Context, id int64) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	Delete(ctx context.Context, id int64) error
}

// CachedUserRepository 使用缓存的 repository 实现。
type CachedUserRepository struct {
	dao   dao.UserDAO
	cache cache.UserCache
}

func NewUserRepository(d dao.UserDAO, c cache.UserCache) UserRepository {
	return &CachedUserRepository{
		dao:   d,
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
	return dao.User{
		ID:       u.ID,
		UserName: u.UserName,
		Email:    sql.NullString{String: u.Email, Valid: u.Email != ""},
		Password: u.Password,
		Phone:    sql.NullString{String: u.Phone, Valid: u.Phone != ""},
		Avatar:   u.Avatar,
	}
}

func (r *CachedUserRepository) entityToDomain(u dao.User) domain.User {
	return domain.User{
		ID:       u.ID,
		UserName: u.UserName,
		Email:    u.Email.String,
		Password: u.Password,
		Phone:    u.Phone.String,
		Avatar:   u.Avatar,
	}
}
