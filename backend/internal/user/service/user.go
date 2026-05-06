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

// 閻劍鍩涢張宥呭
type UserService interface {
	Signup(ctx context.Context, user domain.User) (int64, error)
	Login(ctx context.Context, email, password string) (int64, error)
	// UpdateProfile 閺囧瓨鏌婇棃鐐存櫛閹扮喍淇婇幁顖ょ礄閻劍鍩涢崥宥冣偓浣搞仈閸嶅骏绱?
	UpdateProfile(ctx context.Context, user domain.User) error
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error
	ChangePhone(ctx context.Context, userID int64, password, newPhone string) error
	// Delete 鏉烆垰鍨归梽銈忕礉闂団偓鐟曚線鐛欑拠浣哥槕閻緤绱濋崥灞炬娣囶喗鏁奸柇顔绢唸閸滃本澧滈張鍝勫娇闁灝鍘ら崬顖欑缁便垹绱╅崘鑼崐
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
		return 0, ErrInvalidUserOrPassword // 娑撳秴鎲＄拠澶屾暏閹村嘲鍩屾惔鏇熸Ц闁喚顔堟潻妯绘Ц鐎靛棛鐖滈柨娆掝嚖
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

// UpdateProfile 閺囧瓨鏌婇棃鐐存櫛閹扮喍淇婇幁顖ょ礄閻劍鍩涢崥宥冣偓浣搞仈閸嶅骏绱?
func (svc *userService) UpdateProfile(ctx context.Context, user domain.User) error {
	// 绾喕绻氭稉宥勭窗閺囧瓨鏌婇弫蹇斿妳鐎涙顔?
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

	// 妤犲矁鐦夐弮褍鐦戦惍?
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 閸旂姴鐦戦弬鏉跨槕閻?
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

	// 妤犲矁鐦夌€靛棛鐖?
	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	return svc.repo.Update(ctx, domain.User{ID: userID, Email: newEmail})
}

func (svc *userService) ChangePhone(ctx context.Context, userID int64, password, newPhone string) error {
	panic("change phone is not implemented: sms verification required")
}

// Delete 鏉烆垰鍨归梽銈忕礉妤犲矁鐦夌€靛棛鐖滈崥搴濇叏閺€褰掑仏缁犲崬鎷伴幍瀣簚閸欑兘浼╅崗宥呮暜娑撯偓缁便垹绱╅崘鑼崐
func (svc *userService) Delete(ctx context.Context, userID int64, password string) error {
	u, err := svc.repo.FindById(ctx, userID)
	if err != nil {
		return err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidUserOrPassword
	}

	// 娣囶喗鏁奸柇顔绢唸閸滃本澧滈張鍝勫娇閿涘苯濮炴稉?deleted_ 閸撳秶绱戦崪灞炬闂傚瓨鍩戦敍宀勪缉閸忓秴鏁稉鈧槐銏犵穿閸愯尙鐛?
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
