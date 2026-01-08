package main

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/service"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// UserServiceImpl implements the last service interface defined in the IDL.
type UserServiceImpl struct {
	userService service.UserService
}

// NewUserServiceImpl 创建 UserServiceImpl 实例
func NewUserServiceImpl(userService service.UserService) *UserServiceImpl {
	return &UserServiceImpl{
		userService: userService,
	}
}

// Register implements the UserServiceImpl interface.
func (s *UserServiceImpl) Register(ctx context.Context, req *v1.RegisterReq) (resp *v1.RegisterResp, err error) {
	user := domain.User{
		Email:    req.Email,
		Password: req.Password,
	}
	id, err := s.userService.Signup(ctx, user)
	if err != nil {
		return nil, err
	}
	return &v1.RegisterResp{
		UserId: int32(id),
	}, nil
}

// Login implements the UserServiceImpl interface.
func (s *UserServiceImpl) Login(ctx context.Context, req *v1.LoginReq) (resp *v1.LoginResp, err error) {
	user, err := s.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginResp{
		UserId:   user.ID,
		Username: user.UserName,
		Email:    user.Email,
	}, nil
}

// Logout implements the UserServiceImpl interface.
func (s *UserServiceImpl) Logout(ctx context.Context, req *emptypb.Empty) (resp *emptypb.Empty, err error) {
	// 登出逻辑：可以从 context 中获取用户 ID
	// 这里简单返回空响应，实际业务中可能需要清除 token 等
	return &emptypb.Empty{}, nil
}

// Update implements the UserServiceImpl interface.
func (s *UserServiceImpl) Update(ctx context.Context, req *v1.UpdateUserReq) (resp *emptypb.Empty, err error) {
	user := domain.User{
		ID: req.UserId,
	}
	// 处理 optional 字段
	if req.Username != nil {
		user.UserName = *req.Username
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	// 使用非敏感信息更新，避免更新邮箱等敏感字段
	err = s.userService.UpdateNonSensitiveInfo(ctx, user)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Delete implements the UserServiceImpl interface.
func (s *UserServiceImpl) Delete(ctx context.Context, req *v1.DeleteUserReq) (resp *emptypb.Empty, err error) {
	err = s.userService.Delete(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Query implements the UserServiceImpl interface.
func (s *UserServiceImpl) Query(ctx context.Context, req *v1.QueryUserReq) (resp *v1.QueryUserResp, err error) {
	user, err := s.userService.Query(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &v1.QueryUserResp{
		User: &v1.User{
			UserId:   user.ID,
			Email:    user.Email,
			Username: user.UserName,
			Avatar:   user.Avatar,
		},
	}, nil
}
