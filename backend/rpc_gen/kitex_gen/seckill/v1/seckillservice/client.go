package seckillservice

import (
	"context"

	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	client "github.com/cloudwego/kitex/client"
	callopt "github.com/cloudwego/kitex/client/callopt"
)

type Client interface {
	CreateActivity(ctx context.Context, req *v1.CreateActivityReq, callOptions ...callopt.Option) (r *v1.CreateActivityResp, err error)
	UpdateActivityStatus(ctx context.Context, req *v1.UpdateActivityStatusReq, callOptions ...callopt.Option) (r *v1.UpdateActivityStatusResp, err error)
	GetActivity(ctx context.Context, req *v1.GetActivityReq, callOptions ...callopt.Option) (r *v1.GetActivityResp, err error)
	SubmitSeckill(ctx context.Context, req *v1.SubmitSeckillReq, callOptions ...callopt.Option) (r *v1.SubmitSeckillResp, err error)
	GetSeckillResult(ctx context.Context, req *v1.GetSeckillResultReq, callOptions ...callopt.Option) (r *v1.GetSeckillResultResp, err error)
}

func NewClient(destService string, opts ...client.Option) (Client, error) {
	var options []client.Option
	options = append(options, client.WithDestService(destService))
	options = append(options, opts...)
	kc, err := client.NewClient(serviceInfo(), options...)
	if err != nil {
		return nil, err
	}
	return &kSeckillServiceClient{kClient: newServiceClient(kc)}, nil
}

type kSeckillServiceClient struct{ *kClient }

func (p *kSeckillServiceClient) CreateActivity(ctx context.Context, req *v1.CreateActivityReq, callOptions ...callopt.Option) (*v1.CreateActivityResp, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.CreateActivity(ctx, req)
}
func (p *kSeckillServiceClient) UpdateActivityStatus(ctx context.Context, req *v1.UpdateActivityStatusReq, callOptions ...callopt.Option) (*v1.UpdateActivityStatusResp, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.UpdateActivityStatus(ctx, req)
}
func (p *kSeckillServiceClient) GetActivity(ctx context.Context, req *v1.GetActivityReq, callOptions ...callopt.Option) (*v1.GetActivityResp, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.GetActivity(ctx, req)
}
func (p *kSeckillServiceClient) SubmitSeckill(ctx context.Context, req *v1.SubmitSeckillReq, callOptions ...callopt.Option) (*v1.SubmitSeckillResp, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.SubmitSeckill(ctx, req)
}
func (p *kSeckillServiceClient) GetSeckillResult(ctx context.Context, req *v1.GetSeckillResultReq, callOptions ...callopt.Option) (*v1.GetSeckillResultResp, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.GetSeckillResult(ctx, req)
}
