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

var ErrInvalidUserOrPassword = errors.New("invalid email or password")

type UserService interface {
	Signup(ctx context.Context, user domain.User) (int64, error)
	Login(ctx context.Context, email, password string) (int64, error)
	// UpdateProfile 更新用户资料（不更新邮箱/密码/手机号）。
	UpdateProfile(ctx context.Context, user domain.User) error
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error
	ChangePhone(ctx context.Context, userID int64, password, newPhone string) error
	// Delete 校验密码后删除账号（软删除并打标）。
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
		return 0, ErrInvalidUserOrPassword // 用户不存在也返回统一错误，避免泄露信息
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

// UpdateProfile 更新用户资料（不更新邮箱/密码/手机号）。
func (svc *userService) UpdateProfile(ctx context.Context, user domain.User) error {
	// 清空敏感字段，避免被更新
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

	// 校验旧密码
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 生成新密码哈希
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

	// 校验密码
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	return svc.repo.Update(ctx, domain.User{ID: userID, Email: newEmail})
}

func (svc *userService) ChangePhone(ctx context.Context, userID int64, password, newPhone string) error {
	panic("change phone is not implemented: sms verification required")
}

// Delete 校验密码后软删除账号，并改写邮箱/手机号。
func (svc *userService) Delete(ctx context.Context, userID int64, password string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 用 deleted_ 前缀做标记，避免唯一索引冲突
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
