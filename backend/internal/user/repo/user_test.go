package repo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
	"github.com/stretchr/testify/assert"
)

// MockUserDAO 鏄?dao.UserDAO 鐨?mock 瀹炵幇
type MockUserDAO struct {
	InsertFunc       func(ctx context.Context, u dao.User) (int64, error)
	FindByEmailFunc  func(ctx context.Context, email string) (dao.User, error)
	FindByPhoneFunc  func(ctx context.Context, phone string) (dao.User, error)
	FindByIdFunc     func(ctx context.Context, id int64) (dao.User, error)
	UpdateFunc       func(ctx context.Context, u dao.User) error
	DeleteFunc       func(ctx context.Context, id int64) error
}

func (m *MockUserDAO) Insert(ctx context.Context, u dao.User) (int64, error) {
	return m.InsertFunc(ctx, u)
}

func (m *MockUserDAO) FindByEmail(ctx context.Context, email string) (dao.User, error) {
	return m.FindByEmailFunc(ctx, email)
}

func (m *MockUserDAO) FindByPhone(ctx context.Context, phone string) (dao.User, error) {
	return m.FindByPhoneFunc(ctx, phone)
}

func (m *MockUserDAO) FindById(ctx context.Context, id int64) (dao.User, error) {
	return m.FindByIdFunc(ctx, id)
}

func (m *MockUserDAO) Update(ctx context.Context, u dao.User) error {
	return m.UpdateFunc(ctx, u)
}

func (m *MockUserDAO) Delete(ctx context.Context, id int64) error {
	return m.DeleteFunc(ctx, id)
}

// MockUserCache 鏄?cache.UserCache 鐨?mock 瀹炵幇
type MockUserCache struct{}

func (m *MockUserCache) Get(ctx context.Context, id int64) (dao.User, error) {
	return dao.User{}, dao.ErrDataNotFound
}

func (m *MockUserCache) Set(ctx context.Context, id int64, user dao.User) error {
	return nil
}

func (m *MockUserCache) Delete(ctx context.Context, id int64) error {
	return nil
}

func TestCachedUserRepository_Create(t *testing.T) {
	mockDAO := &MockUserDAO{}
	mockCache := &MockUserCache{}
	repo := NewUserRepository(mockDAO, mockCache)

	tests := []struct {
		name    string
		user    domain.User
		setupFn func()
		wantID  int64
		wantErr error
	}{
		{
			name: "鎴愬姛鍒涘缓鐢ㄦ埛",
			user: domain.User{
				Email:    "test@example.com",
				Password: "hashed_password",
			},
			setupFn: func() {
				mockDAO.InsertFunc = func(ctx context.Context, u dao.User) (int64, error) {
					assert.Equal(t, "test@example.com", u.Email.String)
					assert.True(t, u.Email.Valid)
					return 1, nil
				}
			},
			wantID:  1,
			wantErr: nil,
		},
		{
			name: "閭閲嶅",
			user: domain.User{
				Email:    "duplicate@example.com",
				Password: "hashed_password",
			},
			setupFn: func() {
				mockDAO.InsertFunc = func(ctx context.Context, u dao.User) (int64, error) {
					return 0, dao.ErrUserDuplicate
				}
			},
			wantID:  0,
			wantErr: dao.ErrUserDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			id, err := repo.Create(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestCachedUserRepository_FindByEmail(t *testing.T) {
	mockDAO := &MockUserDAO{}
	mockCache := &MockUserCache{}
	repo := NewUserRepository(mockDAO, mockCache)

	tests := []struct {
		name    string
		email   string
		setupFn func()
		wantErr error
	}{
		{
			name:  "鎴愬姛鏌ユ壘鐢ㄦ埛",
			email: "test@example.com",
			setupFn: func() {
				mockDAO.FindByEmailFunc = func(ctx context.Context, email string) (dao.User, error) {
					assert.Equal(t, "test@example.com", email)
					return dao.User{ID: 1, Email: sql.NullString{String: "test@example.com", Valid: true}}, nil
				}
			},
			wantErr: nil,
		},
		{
			name:  "user not found",
			email: "notfound@example.com",
			setupFn: func() {
				mockDAO.FindByEmailFunc = func(ctx context.Context, email string) (dao.User, error) {
					return dao.User{}, dao.ErrDataNotFound
				}
			},
			wantErr: dao.ErrDataNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			_, err := repo.FindByEmail(context.Background(), tt.email)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestCachedUserRepository_FindById(t *testing.T) {
	mockDAO := &MockUserDAO{}
	mockCache := &MockUserCache{}
	repo := NewUserRepository(mockDAO, mockCache)

	tests := []struct {
		name    string
		id      int64
		setupFn func()
		wantErr error
	}{
		{
			name: "鎴愬姛鏌ユ壘鐢ㄦ埛",
			id:   1,
			setupFn: func() {
				mockDAO.FindByIdFunc = func(ctx context.Context, id int64) (dao.User, error) {
					assert.Equal(t, int64(1), id)
					return dao.User{ID: 1}, nil
				}
			},
			wantErr: nil,
		},
		{
			name: "鐢ㄦ埛涓嶅瓨鍦?,
			id:   999,
			setupFn: func() {
				mockDAO.FindByIdFunc = func(ctx context.Context, id int64) (dao.User, error) {
					return dao.User{}, dao.ErrDataNotFound
				}
			},
			wantErr: dao.ErrDataNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			_, err := repo.FindById(context.Background(), tt.id)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestCachedUserRepository_Update(t *testing.T) {
	mockDAO := &MockUserDAO{}
	mockCache := &MockUserCache{}
	repo := NewUserRepository(mockDAO, mockCache)

	tests := []struct {
		name    string
		user    domain.User
		setupFn func()
		wantErr error
	}{
		{
			name: "鎴愬姛鏇存柊鐢ㄦ埛",
			user: domain.User{
				ID:       1,
				Email:    "updated@example.com",
				Password: "new_password",
			},
			setupFn: func() {
				mockDAO.UpdateFunc = func(ctx context.Context, u dao.User) error {
					assert.Equal(t, int64(1), u.ID)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name: "鐢ㄦ埛涓嶅瓨鍦?,
			user: domain.User{
				ID:    999,
				Email: "notfound@example.com",
			},
			setupFn: func() {
				mockDAO.UpdateFunc = func(ctx context.Context, u dao.User) error {
					return dao.ErrDataNotFound
				}
			},
			wantErr: dao.ErrDataNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := repo.Update(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestCachedUserRepository_Delete(t *testing.T) {
	mockDAO := &MockUserDAO{}
	mockCache := &MockUserCache{}
	repo := NewUserRepository(mockDAO, mockCache)

	tests := []struct {
		name    string
		id      int64
		setupFn func()
		wantErr error
	}{
		{
			name: "鎴愬姛鍒犻櫎鐢ㄦ埛",
			id:   1,
			setupFn: func() {
				mockDAO.DeleteFunc = func(ctx context.Context, id int64) error {
					assert.Equal(t, int64(1), id)
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name: "鐢ㄦ埛涓嶅瓨鍦?,
			id:   999,
			setupFn: func() {
				mockDAO.DeleteFunc = func(ctx context.Context, id int64) error {
					return dao.ErrDataNotFound
				}
			},
			wantErr: dao.ErrDataNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			err := repo.Delete(context.Background(), tt.id)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}


