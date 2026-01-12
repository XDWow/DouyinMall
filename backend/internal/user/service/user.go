package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidUserOrPassword = errors.New("邮箱或密码错误")

// 用户服务
type UserService interface {
	Signup(ctx context.Context, user domain.User) (int64, error)
	Login(ctx context.Context, email, password string) (int64, error)
	// UpdateProfile 更新非敏感信息（用户名、头像）
	UpdateProfile(ctx context.Context, user domain.User) error
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error
	ChangePhone(ctx context.Context, userID int64, password, newPhone string) error
	// Delete 软删除，需要验证密码，同时修改邮箱和手机号避免唯一索引冲突
	Delete(ctx context.Context, userID int64, password string) error
	Query(ctx context.Context, id int64) (domain.User, error)
}

type userService struct {
	repo repo.UserRepository
	l    logger.LoggerV1
}

func NewUserService(repo repo.UserRepository, l logger.LoggerV1) UserService {
	return &userService{
		repo: repo,
		l:    l,
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

func (svc *userService) Login(ctx context.Context, email, password string) (int64, error) {
	u, err := svc.repo.FindByEmail(ctx, email)
	if err == repo.ErrUserNorFound {
		return 0, ErrInvalidUserOrPassword // 不告诉用户到底是邮箱还是密码错误
	}
	if err != nil {
		return 0, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return 0, ErrInvalidUserOrPassword
	}

	return u.ID, nil
}

// UpdateProfile 更新非敏感信息（用户名、头像）
func (svc *userService) UpdateProfile(ctx context.Context, user domain.User) error {
	// 确保不会更新敏感字段
	user.Email = ""
	user.Password = ""
	user.Phone = ""
	return svc.repo.Update(ctx, user)
}

func (svc *userService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 加密新密码
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return svc.repo.Update(ctx, domain.User{ID: userID, Password: string(newHash)})
}

func (svc *userService) ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	// 验证密码
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	return svc.repo.Update(ctx, domain.User{ID: userID, Email: newEmail})
}

func (svc *userService) ChangePhone(ctx context.Context, userID int64, password, newPhone string) error {
	panic("还没实现，等 sms 服务")
}

// Delete 软删除，验证密码后修改邮箱和手机号避免唯一索引冲突
func (svc *userService) Delete(ctx context.Context, userID int64, password string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 修改邮箱和手机号，加上 deleted_ 前缀和时间戳，避免唯一索引冲突
	timestamp := time.Now().Unix()
	deletedEmail := fmt.Sprintf("deleted_%d_%s", timestamp, u.Email)
	deletedPhone := fmt.Sprintf("deleted_%d_%s", timestamp, u.Phone)
	if err = svc.repo.Update(ctx, domain.User{
		ID:    userID,
		Email: deletedEmail,
		Phone: deletedPhone,
	}); err != nil {
		return err
	}

	return svc.repo.Delete(ctx, userID)
}

func (svc *userService) Query(ctx context.Context, id int64) (domain.User, error) {
	return svc.repo.FindById(ctx, id)
}
