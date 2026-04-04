package seckillservice

import (
	"context"
	"errors"

	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	client "github.com/cloudwego/kitex/client"
	kitex "github.com/cloudwego/kitex/pkg/serviceinfo"
	streaming "github.com/cloudwego/kitex/pkg/streaming"
	proto "github.com/cloudwego/prutal"
)

var errInvalidMessageType = errors.New("invalid message type for service method handler")

var serviceMethods = map[string]kitex.MethodInfo{
	"CreateActivity": kitex.NewMethodInfo(createActivityHandler, newCreateActivityArgs, newCreateActivityResult, false, kitex.WithStreamingMode(kitex.StreamingUnary)),
	"UpdateActivityStatus": kitex.NewMethodInfo(updateActivityStatusHandler, newUpdateActivityStatusArgs, newUpdateActivityStatusResult, false, kitex.WithStreamingMode(kitex.StreamingUnary)),
	"GetActivity": kitex.NewMethodInfo(getActivityHandler, newGetActivityArgs, newGetActivityResult, false, kitex.WithStreamingMode(kitex.StreamingUnary)),
	"SubmitSeckill": kitex.NewMethodInfo(submitSeckillHandler, newSubmitSeckillArgs, newSubmitSeckillResult, false, kitex.WithStreamingMode(kitex.StreamingUnary)),
	"GetSeckillResult": kitex.NewMethodInfo(getSeckillResultHandler, newGetSeckillResultArgs, newGetSeckillResultResult, false, kitex.WithStreamingMode(kitex.StreamingUnary)),
}

var (
	seckillServiceServiceInfo                = NewServiceInfo()
	seckillServiceServiceInfoForClient       = NewServiceInfoForClient()
	seckillServiceServiceInfoForStreamClient = NewServiceInfoForStreamClient()
)

func serviceInfo() *kitex.ServiceInfo { return seckillServiceServiceInfo }
func serviceInfoForClient() *kitex.ServiceInfo { return seckillServiceServiceInfoForClient }
func serviceInfoForStreamClient() *kitex.ServiceInfo { return seckillServiceServiceInfoForStreamClient }
func NewServiceInfo() *kitex.ServiceInfo { return newServiceInfo(false, true, true) }
func NewServiceInfoForClient() *kitex.ServiceInfo { return newServiceInfo(false, false, true) }
func NewServiceInfoForStreamClient() *kitex.ServiceInfo { return newServiceInfo(true, true, false) }

func newServiceInfo(hasStreaming, keepStreamingMethods, keepNonStreamingMethods bool) *kitex.ServiceInfo {
	serviceName := "SeckillService"
	handlerType := (*seckillv1.SeckillService)(nil)
	methods := map[string]kitex.MethodInfo{}
	for name, m := range serviceMethods {
		if m.IsStreaming() && !keepStreamingMethods {
			continue
		}
		if !m.IsStreaming() && !keepNonStreamingMethods {
			continue
		}
		methods[name] = m
	}
	extra := map[string]interface{}{"PackageName": "seckill.v1"}
	if hasStreaming {
		extra["streaming"] = hasStreaming
	}
	return &kitex.ServiceInfo{
		ServiceName:     serviceName,
		HandlerType:     handlerType,
		Methods:         methods,
		PayloadCodec:    kitex.Protobuf,
		KiteXGenVersion: "v0.15.4",
		Extra:           extra,
	}
}

type kClient struct{ c client.Client }
func newServiceClient(c client.Client) *kClient { return &kClient{c: c} }

func unaryHandler[Req any, Resp any](ctx context.Context, handler interface{}, arg interface{}, result interface{},
	newReq func() Req,
	call func(seckillv1.SeckillService, context.Context, Req) (Resp, error),
	set func(interface{}, Resp),
) error {
	switch s := arg.(type) {
	case *streaming.Args:
		st := s.Stream
		req := newReq()
		if err := st.RecvMsg(req); err != nil {
			return err
		}
		resp, err := call(handler.(seckillv1.SeckillService), ctx, req)
		if err != nil {
			return err
		}
		return st.SendMsg(resp)
	case interface{ GetFirstArgument() interface{} }:
		req := s.GetFirstArgument().(Req)
		resp, err := call(handler.(seckillv1.SeckillService), ctx, req)
		if err != nil {
			return err
		}
		set(result, resp)
		return nil
	default:
		return errInvalidMessageType
	}
}

type CreateActivityArgs struct{ Req *v1.CreateActivityReq }
func newCreateActivityArgs() interface{} { return &CreateActivityArgs{} }
func (p *CreateActivityArgs) Marshal(out []byte) ([]byte, error) { if p.Req == nil { return out, nil }; return proto.Marshal(p.Req) }
func (p *CreateActivityArgs) Unmarshal(in []byte) error { msg := new(v1.CreateActivityReq); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Req = msg; return nil }
func (p *CreateActivityArgs) GetFirstArgument() interface{} { return p.Req }
type CreateActivityResult struct{ Success *v1.CreateActivityResp }
func newCreateActivityResult() interface{} { return &CreateActivityResult{} }
func (p *CreateActivityResult) Marshal(out []byte) ([]byte, error) { if p.Success == nil { return out, nil }; return proto.Marshal(p.Success) }
func (p *CreateActivityResult) Unmarshal(in []byte) error { msg := new(v1.CreateActivityResp); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Success = msg; return nil }
func createActivityHandler(ctx context.Context, handler interface{}, arg, result interface{}) error {
	return unaryHandler[*v1.CreateActivityReq, *v1.CreateActivityResp](ctx, handler, arg, result, func() *v1.CreateActivityReq { return new(v1.CreateActivityReq) }, func(s seckillv1.SeckillService, ctx context.Context, req *v1.CreateActivityReq) (*v1.CreateActivityResp, error) { return s.CreateActivity(ctx, req) }, func(result interface{}, resp *v1.CreateActivityResp) { result.(*CreateActivityResult).Success = resp })
}

type UpdateActivityStatusArgs struct{ Req *v1.UpdateActivityStatusReq }
func newUpdateActivityStatusArgs() interface{} { return &UpdateActivityStatusArgs{} }
func (p *UpdateActivityStatusArgs) Marshal(out []byte) ([]byte, error) { if p.Req == nil { return out, nil }; return proto.Marshal(p.Req) }
func (p *UpdateActivityStatusArgs) Unmarshal(in []byte) error { msg := new(v1.UpdateActivityStatusReq); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Req = msg; return nil }
func (p *UpdateActivityStatusArgs) GetFirstArgument() interface{} { return p.Req }
type UpdateActivityStatusResult struct{ Success *v1.UpdateActivityStatusResp }
func newUpdateActivityStatusResult() interface{} { return &UpdateActivityStatusResult{} }
func (p *UpdateActivityStatusResult) Marshal(out []byte) ([]byte, error) { if p.Success == nil { return out, nil }; return proto.Marshal(p.Success) }
func (p *UpdateActivityStatusResult) Unmarshal(in []byte) error { msg := new(v1.UpdateActivityStatusResp); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Success = msg; return nil }
func updateActivityStatusHandler(ctx context.Context, handler interface{}, arg, result interface{}) error {
	return unaryHandler[*v1.UpdateActivityStatusReq, *v1.UpdateActivityStatusResp](ctx, handler, arg, result, func() *v1.UpdateActivityStatusReq { return new(v1.UpdateActivityStatusReq) }, func(s seckillv1.SeckillService, ctx context.Context, req *v1.UpdateActivityStatusReq) (*v1.UpdateActivityStatusResp, error) { return s.UpdateActivityStatus(ctx, req) }, func(result interface{}, resp *v1.UpdateActivityStatusResp) { result.(*UpdateActivityStatusResult).Success = resp })
}

type GetActivityArgs struct{ Req *v1.GetActivityReq }
func newGetActivityArgs() interface{} { return &GetActivityArgs{} }
func (p *GetActivityArgs) Marshal(out []byte) ([]byte, error) { if p.Req == nil { return out, nil }; return proto.Marshal(p.Req) }
func (p *GetActivityArgs) Unmarshal(in []byte) error { msg := new(v1.GetActivityReq); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Req = msg; return nil }
func (p *GetActivityArgs) GetFirstArgument() interface{} { return p.Req }
type GetActivityResult struct{ Success *v1.GetActivityResp }
func newGetActivityResult() interface{} { return &GetActivityResult{} }
func (p *GetActivityResult) Marshal(out []byte) ([]byte, error) { if p.Success == nil { return out, nil }; return proto.Marshal(p.Success) }
func (p *GetActivityResult) Unmarshal(in []byte) error { msg := new(v1.GetActivityResp); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Success = msg; return nil }
func getActivityHandler(ctx context.Context, handler interface{}, arg, result interface{}) error {
	return unaryHandler[*v1.GetActivityReq, *v1.GetActivityResp](ctx, handler, arg, result, func() *v1.GetActivityReq { return new(v1.GetActivityReq) }, func(s seckillv1.SeckillService, ctx context.Context, req *v1.GetActivityReq) (*v1.GetActivityResp, error) { return s.GetActivity(ctx, req) }, func(result interface{}, resp *v1.GetActivityResp) { result.(*GetActivityResult).Success = resp })
}

type SubmitSeckillArgs struct{ Req *v1.SubmitSeckillReq }
func newSubmitSeckillArgs() interface{} { return &SubmitSeckillArgs{} }
func (p *SubmitSeckillArgs) Marshal(out []byte) ([]byte, error) { if p.Req == nil { return out, nil }; return proto.Marshal(p.Req) }
func (p *SubmitSeckillArgs) Unmarshal(in []byte) error { msg := new(v1.SubmitSeckillReq); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Req = msg; return nil }
func (p *SubmitSeckillArgs) GetFirstArgument() interface{} { return p.Req }
type SubmitSeckillResult struct{ Success *v1.SubmitSeckillResp }
func newSubmitSeckillResult() interface{} { return &SubmitSeckillResult{} }
func (p *SubmitSeckillResult) Marshal(out []byte) ([]byte, error) { if p.Success == nil { return out, nil }; return proto.Marshal(p.Success) }
func (p *SubmitSeckillResult) Unmarshal(in []byte) error { msg := new(v1.SubmitSeckillResp); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Success = msg; return nil }
func submitSeckillHandler(ctx context.Context, handler interface{}, arg, result interface{}) error {
	return unaryHandler[*v1.SubmitSeckillReq, *v1.SubmitSeckillResp](ctx, handler, arg, result, func() *v1.SubmitSeckillReq { return new(v1.SubmitSeckillReq) }, func(s seckillv1.SeckillService, ctx context.Context, req *v1.SubmitSeckillReq) (*v1.SubmitSeckillResp, error) { return s.SubmitSeckill(ctx, req) }, func(result interface{}, resp *v1.SubmitSeckillResp) { result.(*SubmitSeckillResult).Success = resp })
}

type GetSeckillResultArgs struct{ Req *v1.GetSeckillResultReq }
func newGetSeckillResultArgs() interface{} { return &GetSeckillResultArgs{} }
func (p *GetSeckillResultArgs) Marshal(out []byte) ([]byte, error) { if p.Req == nil { return out, nil }; return proto.Marshal(p.Req) }
func (p *GetSeckillResultArgs) Unmarshal(in []byte) error { msg := new(v1.GetSeckillResultReq); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Req = msg; return nil }
func (p *GetSeckillResultArgs) GetFirstArgument() interface{} { return p.Req }
type GetSeckillResultResult struct{ Success *v1.GetSeckillResultResp }
func newGetSeckillResultResult() interface{} { return &GetSeckillResultResult{} }
func (p *GetSeckillResultResult) Marshal(out []byte) ([]byte, error) { if p.Success == nil { return out, nil }; return proto.Marshal(p.Success) }
func (p *GetSeckillResultResult) Unmarshal(in []byte) error { msg := new(v1.GetSeckillResultResp); if err := proto.Unmarshal(in, msg); err != nil { return err }; p.Success = msg; return nil }
func getSeckillResultHandler(ctx context.Context, handler interface{}, arg, result interface{}) error {
	return unaryHandler[*v1.GetSeckillResultReq, *v1.GetSeckillResultResp](ctx, handler, arg, result, func() *v1.GetSeckillResultReq { return new(v1.GetSeckillResultReq) }, func(s seckillv1.SeckillService, ctx context.Context, req *v1.GetSeckillResultReq) (*v1.GetSeckillResultResp, error) { return s.GetSeckillResult(ctx, req) }, func(result interface{}, resp *v1.GetSeckillResultResp) { result.(*GetSeckillResultResult).Success = resp })
}

func (p *kClient) CreateActivity(ctx context.Context, req *v1.CreateActivityReq) (*v1.CreateActivityResp, error) {
	var args CreateActivityArgs
	args.Req = req
	var result CreateActivityResult
	if err := p.c.Call(ctx, "CreateActivity", &args, &result); err != nil {
		return nil, err
	}
	return result.Success, nil
}
func (p *kClient) UpdateActivityStatus(ctx context.Context, req *v1.UpdateActivityStatusReq) (*v1.UpdateActivityStatusResp, error) {
	var args UpdateActivityStatusArgs
	args.Req = req
	var result UpdateActivityStatusResult
	if err := p.c.Call(ctx, "UpdateActivityStatus", &args, &result); err != nil {
		return nil, err
	}
	return result.Success, nil
}
func (p *kClient) GetActivity(ctx context.Context, req *v1.GetActivityReq) (*v1.GetActivityResp, error) {
	var args GetActivityArgs
	args.Req = req
	var result GetActivityResult
	if err := p.c.Call(ctx, "GetActivity", &args, &result); err != nil {
		return nil, err
	}
	return result.Success, nil
}
func (p *kClient) SubmitSeckill(ctx context.Context, req *v1.SubmitSeckillReq) (*v1.SubmitSeckillResp, error) {
	var args SubmitSeckillArgs
	args.Req = req
	var result SubmitSeckillResult
	if err := p.c.Call(ctx, "SubmitSeckill", &args, &result); err != nil {
		return nil, err
	}
	return result.Success, nil
}
func (p *kClient) GetSeckillResult(ctx context.Context, req *v1.GetSeckillResultReq) (*v1.GetSeckillResultResp, error) {
	var args GetSeckillResultArgs
	args.Req = req
	var result GetSeckillResultResult
	if err := p.c.Call(ctx, "GetSeckillResult", &args, &result); err != nil {
		return nil, err
	}
	return result.Success, nil
}


