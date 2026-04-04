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

var ErrInvalidUserOrPassword = errors.New("閭鎴栧瘑鐮侀敊璇?)

// 鐢ㄦ埛鏈嶅姟
type UserService interface {
	Signup(ctx context.Context, user domain.User) (int64, error)
	Login(ctx context.Context, email, password string) (int64, error)
	// UpdateProfile 鏇存柊闈炴晱鎰熶俊鎭紙鐢ㄦ埛鍚嶃€佸ご鍍忥級
	UpdateProfile(ctx context.Context, user domain.User) error
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error
	ChangePhone(ctx context.Context, userID int64, password, newPhone string) error
	// Delete 杞垹闄わ紝闇€瑕侀獙璇佸瘑鐮侊紝鍚屾椂淇敼閭鍜屾墜鏈哄彿閬垮厤鍞竴绱㈠紩鍐茬獊
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
		return 0, ErrInvalidUserOrPassword // 涓嶅憡璇夌敤鎴峰埌搴曟槸閭杩樻槸瀵嗙爜閿欒
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

// UpdateProfile 鏇存柊闈炴晱鎰熶俊鎭紙鐢ㄦ埛鍚嶃€佸ご鍍忥級
func (svc *userService) UpdateProfile(ctx context.Context, user domain.User) error {
	// 纭繚涓嶄細鏇存柊鏁忔劅瀛楁
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

	// 楠岃瘉鏃у瘑鐮?
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 鍔犲瘑鏂板瘑鐮?
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

	// 楠岃瘉瀵嗙爜
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	return svc.repo.Update(ctx, domain.User{ID: userID, Email: newEmail})
}

func (svc *userService) ChangePhone(ctx context.Context, userID int64, password, newPhone string) error {
	panic("杩樻病瀹炵幇锛岀瓑 sms 鏈嶅姟")
}

// Delete 杞垹闄わ紝楠岃瘉瀵嗙爜鍚庝慨鏀归偖绠卞拰鎵嬫満鍙烽伩鍏嶅敮涓€绱㈠紩鍐茬獊
func (svc *userService) Delete(ctx context.Context, userID int64, password string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 淇敼閭鍜屾墜鏈哄彿锛屽姞涓?deleted_ 鍓嶇紑鍜屾椂闂存埑锛岄伩鍏嶅敮涓€绱㈠紩鍐茬獊
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


