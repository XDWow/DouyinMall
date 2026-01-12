package handler

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"github.com/XDWow/DouyinMall/backend/internal/user/service"
	userv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}

// MockUserService 是 service.UserService 的 mock 实现
type MockUserService struct {
	SignupFunc         func(ctx context.Context, user domain.User) (int64, error)
	LoginFunc          func(ctx context.Context, email, password string) (int64, error)
	QueryFunc          func(ctx context.Context, id int64) (domain.User, error)
	UpdateProfileFunc  func(ctx context.Context, user domain.User) error
	ChangePasswordFunc func(ctx context.Context, userID int64, oldPassword, newPassword string) error
	ChangeEmailFunc    func(ctx context.Context, userID int64, password, newEmail string) error
	ChangePhoneFunc    func(ctx context.Context, userID int64, password, newPhone string) error
	DeleteFunc         func(ctx context.Context, id int64, password string) error
}

func (m *MockUserService) Signup(ctx context.Context, user domain.User) (int64, error) {
	return m.SignupFunc(ctx, user)
}

func (m *MockUserService) Login(ctx context.Context, email, password string) (int64, error) {
	return m.LoginFunc(ctx, email, password)
}

func (m *MockUserService) Query(ctx context.Context, id int64) (domain.User, error) {
	return m.QueryFunc(ctx, id)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, user domain.User) error {
	return m.UpdateProfileFunc(ctx, user)
}

func (m *MockUserService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	return m.ChangePasswordFunc(ctx, userID, oldPassword, newPassword)
}

func (m *MockUserService) ChangeEmail(ctx context.Context, userID int64, password, newEmail string) error {
	return m.ChangeEmailFunc(ctx, userID, password, newEmail)
}

func (m *MockUserService) ChangePhone(ctx context.Context, userID int64, password, newPhone string) error {
	return m.ChangePhoneFunc(ctx, userID, password, newPhone)
}

func (m *MockUserService) Delete(ctx context.Context, id int64, password string) error {
	return m.DeleteFunc(ctx, id, password)
}

func TestUserServiceServer_Signup(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.SignupReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功注册",
			req: &userv1.SignupReq{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupFn: func() {
				mockSvc.SignupFunc = func(ctx context.Context, user domain.User) (int64, error) {
					assert.Equal(t, "test@example.com", user.Email)
					assert.Equal(t, "password123", user.Password)
					return 1, nil
				}
			},
			wantErr: false,
		},
		{
			name: "注册失败-邮箱重复",
			req: &userv1.SignupReq{
				Email:    "duplicate@example.com",
				Password: "password123",
			},
			setupFn: func() {
				mockSvc.SignupFunc = func(ctx context.Context, user domain.User) (int64, error) {
					return 0, repo.ErrUserDuplicateEmail
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.Signup(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(1), resp.UserId)
			}
		})
	}
}

func TestUserServiceServer_Login(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.LoginReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功登录",
			req: &userv1.LoginReq{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupFn: func() {
				mockSvc.LoginFunc = func(ctx context.Context, email, password string) (int64, error) {
					assert.Equal(t, "test@example.com", email)
					assert.Equal(t, "password123", password)
					return 1, nil
				}
			},
			wantErr: false,
		},
		{
			name: "登录失败-用户不存在或密码错误",
			req: &userv1.LoginReq{
				Email:    "notfound@example.com",
				Password: "wrongpassword",
			},
			setupFn: func() {
				mockSvc.LoginFunc = func(ctx context.Context, email, password string) (int64, error) {
					return 0, service.ErrInvalidUserOrPassword
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.Login(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(1), resp.UserId)
			}
		})
	}
}

func TestUserServiceServer_Query(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	user := domain.User{
		ID:       1,
		Email:    "test@example.com",
		UserName: "TestUser",
	}

	tests := []struct {
		name    string
		req     *userv1.QueryUserReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功获取用户信息",
			req: &userv1.QueryUserReq{
				UserId: 1,
			},
			setupFn: func() {
				mockSvc.QueryFunc = func(ctx context.Context, id int64) (domain.User, error) {
					assert.Equal(t, int64(1), id)
					return user, nil
				}
			},
			wantErr: false,
		},
		{
			name: "用户不存在",
			req: &userv1.QueryUserReq{
				UserId: 999,
			},
			setupFn: func() {
				mockSvc.QueryFunc = func(ctx context.Context, id int64) (domain.User, error) {
					return domain.User{}, repo.ErrUserNorFound
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.Query(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.User)
				assert.Equal(t, int64(1), resp.User.UserId)
				assert.Equal(t, "test@example.com", resp.User.Email)
			}
		})
	}
}

func TestUserServiceServer_UpdateProfile(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.UpdateProfileReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功更新",
			req: &userv1.UpdateProfileReq{
				UserId:   1,
				Username: stringPtr("NewUserName"),
			},
			setupFn: func() {
				mockSvc.UpdateProfileFunc = func(ctx context.Context, user domain.User) error {
					assert.Equal(t, int64(1), user.ID)
					assert.Equal(t, "NewUserName", user.UserName)
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "用户不存在",
			req: &userv1.UpdateProfileReq{
				UserId:   999,
				Username: stringPtr("NewUserName"),
			},
			setupFn: func() {
				mockSvc.UpdateProfileFunc = func(ctx context.Context, user domain.User) error {
					return repo.ErrUserNorFound
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.UpdateProfile(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, &emptypb.Empty{}, resp)
			}
		})
	}
}

func TestUserServiceServer_ChangePassword(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.ChangePasswordReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功修改密码",
			req: &userv1.ChangePasswordReq{
				UserId:      1,
				OldPassword: "oldpassword",
				NewPassword: "newpassword",
			},
			setupFn: func() {
				mockSvc.ChangePasswordFunc = func(ctx context.Context, userID int64, oldPassword, newPassword string) error {
					assert.Equal(t, int64(1), userID)
					assert.Equal(t, "oldpassword", oldPassword)
					assert.Equal(t, "newpassword", newPassword)
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "旧密码错误",
			req: &userv1.ChangePasswordReq{
				UserId:      1,
				OldPassword: "wrongpassword",
				NewPassword: "newpassword",
			},
			setupFn: func() {
				mockSvc.ChangePasswordFunc = func(ctx context.Context, userID int64, oldPassword, newPassword string) error {
					return service.ErrInvalidUserOrPassword
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.ChangePassword(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, &emptypb.Empty{}, resp)
			}
		})
	}
}

func TestUserServiceServer_ChangeEmail(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.ChangeEmailReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功修改邮箱",
			req: &userv1.ChangeEmailReq{
				UserId:   1,
				Password: "password",
				NewEmail: "newemail@example.com",
			},
			setupFn: func() {
				mockSvc.ChangeEmailFunc = func(ctx context.Context, userID int64, password, newEmail string) error {
					assert.Equal(t, int64(1), userID)
					assert.Equal(t, "password", password)
					assert.Equal(t, "newemail@example.com", newEmail)
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "密码错误",
			req: &userv1.ChangeEmailReq{
				UserId:   1,
				Password: "wrongpassword",
				NewEmail: "newemail@example.com",
			},
			setupFn: func() {
				mockSvc.ChangeEmailFunc = func(ctx context.Context, userID int64, password, newEmail string) error {
					return service.ErrInvalidUserOrPassword
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.ChangeEmail(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, &emptypb.Empty{}, resp)
			}
		})
	}
}

func TestUserServiceServer_Delete(t *testing.T) {
	mockSvc := &MockUserService{}
	server := NewUserServiceServer(mockSvc)

	tests := []struct {
		name    string
		req     *userv1.DeleteUserReq
		setupFn func()
		wantErr bool
	}{
		{
			name: "成功删除",
			req: &userv1.DeleteUserReq{
				UserId:   1,
				Password: "password",
			},
			setupFn: func() {
				mockSvc.DeleteFunc = func(ctx context.Context, id int64, password string) (error) {
					assert.Equal(t, int64(1), id)
					assert.Equal(t, "password", password)
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "密码错误",
			req: &userv1.DeleteUserReq{
				UserId:   1,
				Password: "wrongpassword",
			},
			setupFn: func() {
				mockSvc.DeleteFunc = func(ctx context.Context, id int64, password string) error {
					return service.ErrInvalidUserOrPassword
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()
			resp, err := server.Delete(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, &emptypb.Empty{}, resp)
			}
		})
	}
}
