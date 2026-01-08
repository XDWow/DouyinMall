package service

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

var ErrInvalidUserOrPassword = errors.New("邮箱或密码错误")

// 用户服务
type UserService interface {
	Signup(ctx context.Context, user domain.User) (int64, error)
	Login(ctx context.Context, email, password string) (domain.User, error)
	Logout(ctx context.Context, userID int64) error
	Update(ctx context.Context, user domain.User) error
	UpdateNonSensitiveInfo(ctx context.Context, user domain.User) error
	Delete(ctx context.Context, id int64) error
	Query(ctx context.Context, id int64) (domain.User, error)
}

type userService struct {
	repo   repo.UserRepository
	l      logger.LoggerV1
}

func NewUserService(repo repo.UserRepository, l logger.LoggerV1) UserService {
	return &userService{
		repo:   repo,
		l: 		l,
	}
}

func (svc *userService) Signup(ctx context.Context, user domain.User) (int64, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	user.Password = string(passwordHash)
	return svc.repo.Create(ctx, user)
}

func (svc *userService) Login(ctx context.Context, email, password string) (domain.User, error) {
	u, err := svc.repo.FindByEmail(ctx, email)
	if err == repo.ErrUserNorFound {
		return domain.User{}, ErrInvalidUserOrPassword // 这个设计就是，不告诉你到底是邮箱还是密码错误
	}
	if err != nil {
		return domain.User{}, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return domain.User{}, ErrInvalidUserOrPassword
	}
	return u, nil
}

func (svc *userService) Logout(ctx context.Context, userID int64) error {
	// 登出逻辑：可以在这里处理 token 失效、清除缓存等操作
	
}

// Update 更新用户信息
func (svc *userService) Update(ctx context.Context, user domain.User) error {
	return svc.repo.Update(ctx, user)
}

// UpdateNonSensitiveInfo 只能更新非敏感信息
func (svc *userService) UpdateNonSensitiveInfo(ctx context.Context, user domain.User) error {
	// 在 service 层面上维护住了什么是敏感字段这个语义
	// 依赖 repo 中更新会忽略零值
	user.Email = ""
	user.Password = ""
	user.Phone = ""
	return svc.repo.Update(ctx, user)
}

func (svc *userService) Delete(ctx context.Context, ID int64) error {
	return svc.repo.Delete(ctx, ID)
}

func (svc *userService) Query(ctx context.Context, id int64) (domain.User, error) {
	return svc.repo.FindById(ctx, id)
}
