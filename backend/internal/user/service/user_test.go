package service

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

// MockUserRepository 鏄?repo.UserRepository 鐨?mock 瀹炵幇
type MockUserRepository struct {
	CreateFunc      func(ctx context.Context, u domain.User) (int64, error)
	FindByEmailFunc func(ctx context.Context, email string) (domain.User, error)
	FindByIdFunc    func(ctx context.Context, id int64) (domain.User, error)
	UpdateFunc      func(ctx context.Context, u domain.User) error
	DeleteFunc      func(ctx context.Context, id int64) error
}

func (m *MockUserRepository) Create(ctx context.Context, u domain.User) (int64, error) {
	return m.CreateFunc(ctx, u)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return m.FindByEmailFunc(ctx, email)
}

func (m *MockUserRepository) FindById(ctx context.Context, id int64) (domain.User, error) {
	return m.FindByIdFunc(ctx, id)
}

func (m *MockUserRepository) Update(ctx context.Context, u domain.User) error {
	return m.UpdateFunc(ctx, u)
}

func (m *MockUserRepository) Delete(ctx context.Context, id int64) error {
	return m.DeleteFunc(ctx, id)
}

func newMockLogger() logger.LoggerV1 {
	l, _ := zap.NewDevelopment()
	return logger.NewZapLogger(l)
}

func TestUserService_Signup(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	tests := []struct {
		name    string
		user    domain.User
		setupFn func()
		wantErr error
	}{
		{
			name: "鎴愬姛娉ㄥ唽",
			user: domain.User{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupFn: func() {
				mockRepo.CreateFunc = func(ctx context.Context, u domain.User) (int64, error) {
					assert.Equal(t, "test@example.com", u.Email)
					// 楠岃瘉瀵嗙爜宸茶鍔犲瘑
					err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("password123"))
					assert.NoError(t, err)
					return 1, nil
				}
			},
			wantErr: nil,
		},
		{
			name: "閭宸插瓨鍦?,
			user: domain.User{
				Email:    "duplicate@example.com",
				Password: "password123",
			},
			setupFn: func() {
				mockRepo.CreateFunc = func(ctx context.Context, u domain.User) (int64, error) {
					return 0, repo.ErrUserDuplicateEmail
				}
			},
			wantErr: repo.ErrUserDuplicateEmail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			_, err := svc.Signup(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestUserService_Login(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	// 鐢熸垚涓€涓凡鍔犲瘑鐨勫瘑鐮?
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	tests := []struct {
		name     string
		email    string
		password string
		setupFn  func()
		wantErr  error
	}{
		{
			name:     "鎴愬姛鐧诲綍",
			email:    "test@example.com",
			password: "correctpassword",
			setupFn: func() {
				mockRepo.FindByEmailFunc = func(ctx context.Context, email string) (domain.User, error) {
					assert.Equal(t, "test@example.com", email)
					return domain.User{
						ID:       1,
						Email:    "test@example.com",
						Password: string(hashedPassword),
					}, nil
				}
			},
			wantErr: nil,
		},
		{
			name:     "鐢ㄦ埛涓嶅瓨鍦?,
			email:    "notfound@example.com",
			password: "password",
			setupFn: func() {
				mockRepo.FindByEmailFunc = func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, repo.ErrUserNorFound
				}
			},
			wantErr: ErrInvalidUserOrPassword,
		},
		{
			name:     "瀵嗙爜閿欒",
			email:    "test@example.com",
			password: "wrongpassword",
			setupFn: func() {
				mockRepo.FindByEmailFunc = func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{
						ID:       1,
						Email:    "test@example.com",
						Password: string(hashedPassword),
					}, nil
				}
			},
			wantErr: ErrInvalidUserOrPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			userID, err := svc.Login(context.Background(), tt.email, tt.password)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, int64(1), userID)
			}
		})
	}
}

func TestUserService_Query(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	tests := []struct {
		name     string
		id       int64
		setupFn  func()
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "鎴愬姛鑾峰彇鐢ㄦ埛淇℃伅",
			id:   1,
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					assert.Equal(t, int64(1), id)
					return domain.User{ID: 1, Email: "test@example.com"}, nil
				}
			},
			wantUser: domain.User{ID: 1, Email: "test@example.com"},
			wantErr:  nil,
		},
		{
			name: "鐢ㄦ埛涓嶅瓨鍦?,
			id:   999,
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{}, repo.ErrUserNorFound
				}
			},
			wantUser: domain.User{},
			wantErr:  repo.ErrUserNorFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			user, err := svc.Query(context.Background(), tt.id)
			assert.Equal(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.wantUser.ID, user.ID)
				assert.Equal(t, tt.wantUser.Email, user.Email)
			}
		})
	}
}

func TestUserService_UpdateProfile(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	tests := []struct {
		name    string
		user    domain.User
		setupFn func()
		wantErr error
	}{
		{
			name: "鎴愬姛鏇存柊",
			user: domain.User{
				ID:       1,
				UserName: "NewUserName",
			},
			setupFn: func() {
				mockRepo.UpdateFunc = func(ctx context.Context, u domain.User) error {
					assert.Equal(t, int64(1), u.ID)
					assert.Equal(t, "NewUserName", u.UserName)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name: "鐢ㄦ埛涓嶅瓨鍦?,
			user: domain.User{
				ID:       999,
				UserName: "NewUserName",
			},
			setupFn: func() {
				mockRepo.UpdateFunc = func(ctx context.Context, u domain.User) error {
					return repo.ErrUserNorFound
				}
			},
			wantErr: repo.ErrUserNorFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := svc.UpdateProfile(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	// 鐢熸垚宸插姞瀵嗙殑鏃у瘑鐮?
	oldHashedPassword, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.DefaultCost)

	tests := []struct {
		name        string
		userID      int64
		oldPassword string
		newPassword string
		setupFn     func()
		wantErr     error
	}{
		{
			name:        "鎴愬姛淇敼瀵嗙爜",
			userID:      1,
			oldPassword: "oldpassword",
			newPassword: "newpassword",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Password: string(oldHashedPassword),
					}, nil
				}
				mockRepo.UpdateFunc = func(ctx context.Context, u domain.User) error {
					assert.Equal(t, int64(1), u.ID)
					// 楠岃瘉鏂板瘑鐮佸凡琚姞瀵?
					err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("newpassword"))
					assert.NoError(t, err)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:        "鏃у瘑鐮侀敊璇?,
			userID:      1,
			oldPassword: "wrongpassword",
			newPassword: "newpassword",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Password: string(oldHashedPassword),
					}, nil
				}
			},
			wantErr: ErrInvalidUserOrPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := svc.ChangePassword(context.Background(), tt.userID, tt.oldPassword, tt.newPassword)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestUserService_ChangeEmail(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)

	tests := []struct {
		name     string
		userID   int64
		password string
		newEmail string
		setupFn  func()
		wantErr  error
	}{
		{
			name:     "鎴愬姛淇敼閭",
			userID:   1,
			password: "password",
			newEmail: "newemail@example.com",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Email:    "old@example.com",
						Password: string(hashedPassword),
					}, nil
				}
				mockRepo.UpdateFunc = func(ctx context.Context, u domain.User) error {
					assert.Equal(t, int64(1), u.ID)
					assert.Equal(t, "newemail@example.com", u.Email)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:     "瀵嗙爜閿欒",
			userID:   1,
			password: "wrongpassword",
			newEmail: "newemail@example.com",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Password: string(hashedPassword),
					}, nil
				}
			},
			wantErr: ErrInvalidUserOrPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := svc.ChangeEmail(context.Background(), tt.userID, tt.password, tt.newEmail)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestUserService_Delete(t *testing.T) {
	mockRepo := &MockUserRepository{}
	mockLogger := newMockLogger()
	svc := NewUserService(mockRepo, mockLogger)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)

	tests := []struct {
		name     string
		userID   int64
		password string
		setupFn  func()
		wantErr  error
	}{
		{
			name:     "鎴愬姛鍒犻櫎璐﹀彿",
			userID:   1,
			password: "password",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Email:    "test@example.com",
						Phone:    "13800138000",
						Password: string(hashedPassword),
					}, nil
				}
				mockRepo.UpdateFunc = func(ctx context.Context, u domain.User) error {
					// 楠岃瘉閭鍜屾墜鏈哄彿宸茶鏍囪涓哄垹闄?
					assert.Contains(t, u.Email, "deleted_")
					assert.Contains(t, u.Phone, "deleted_")
					return nil
				}
				mockRepo.DeleteFunc = func(ctx context.Context, id int64) error {
					assert.Equal(t, int64(1), id)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:     "瀵嗙爜閿欒",
			userID:   1,
			password: "wrongpassword",
			setupFn: func() {
				mockRepo.FindByIdFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{
						ID:       1,
						Password: string(hashedPassword),
					}, nil
				}
			},
			wantErr: ErrInvalidUserOrPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := svc.Delete(context.Background(), tt.userID, tt.password)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}


