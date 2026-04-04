package handler

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/user/domain"
	"github.com/XDWow/DouyinMall/backend/internal/user/service"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type UserServiceServer struct {
	userService service.UserService
}

func NewUserServiceServer(userService service.UserService) *UserServiceServer {
	return &UserServiceServer{
		userService: userService,
	}
}

func (s *UserServiceServer) Signup(ctx context.Context, req *v1.SignupReq) (resp *v1.SignupResp, err error) {
	user := domain.User{
		Email:    req.Email,
		Password: req.Password,
	}
	id, err := s.userService.Signup(ctx, user)
	if err != nil {
		return nil, err
	}
	return &v1.SignupResp{
		UserId: id,
	}, nil
}

func (s *UserServiceServer) Login(ctx context.Context, req *v1.LoginReq) (resp *v1.LoginResp, err error) {
	userID, err := s.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginResp{
		UserId: userID,
	}, nil
}

func (s *UserServiceServer) UpdateProfile(ctx context.Context, req *v1.UpdateProfileReq) (resp *emptypb.Empty, err error) {
	user := domain.User{ID: req.UserId}
	if req.Username != nil {
		user.UserName = *req.Username
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	err = s.userService.UpdateProfile(ctx, user)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServiceServer) ChangePassword(ctx context.Context, req *v1.ChangePasswordReq) (resp *emptypb.Empty, err error) {
	err = s.userService.ChangePassword(ctx, req.UserId, req.OldPassword, req.NewPassword)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServiceServer) ChangeEmail(ctx context.Context, req *v1.ChangeEmailReq) (resp *emptypb.Empty, err error) {
	err = s.userService.ChangeEmail(ctx, req.UserId, req.Password, req.NewEmail)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServiceServer) ChangePhone(ctx context.Context, req *v1.ChangePhoneReq) (resp *emptypb.Empty, err error) {
	err = s.userService.ChangePhone(ctx, req.UserId, req.OldPhone, req.NewPhone)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServiceServer) Delete(ctx context.Context, req *v1.DeleteUserReq) (resp *emptypb.Empty, err error) {
	err = s.userService.Delete(ctx, req.UserId, req.Password)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServiceServer) Query(ctx context.Context, req *v1.QueryUserReq) (resp *v1.QueryUserResp, err error) {
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


