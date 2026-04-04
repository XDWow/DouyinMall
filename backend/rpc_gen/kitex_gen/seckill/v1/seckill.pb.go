package seckillv1

import (
	"context"

	"github.com/cloudwego/prutal"
)

type SeckillActivity struct {
	Id             int64  `protobuf:"varint,1,opt,name=id" json:"id,omitempty"`
	ActivityName   string `protobuf:"bytes,2,opt,name=activity_name" json:"activity_name,omitempty"`
	ProductId      int64  `protobuf:"varint,3,opt,name=product_id" json:"product_id,omitempty"`
	SkuId          int64  `protobuf:"varint,4,opt,name=sku_id" json:"sku_id,omitempty"`
	SeckillPrice   int64  `protobuf:"varint,5,opt,name=seckill_price" json:"seckill_price,omitempty"`
	TotalStock     int32  `protobuf:"varint,6,opt,name=total_stock" json:"total_stock,omitempty"`
	AvailableStock int32  `protobuf:"varint,7,opt,name=available_stock" json:"available_stock,omitempty"`
	StartTime      int64  `protobuf:"varint,8,opt,name=start_time" json:"start_time,omitempty"`
	EndTime        int64  `protobuf:"varint,9,opt,name=end_time" json:"end_time,omitempty"`
	Status         string `protobuf:"bytes,10,opt,name=status" json:"status,omitempty"`
	LimitPerUser   int32  `protobuf:"varint,11,opt,name=limit_per_user" json:"limit_per_user,omitempty"`
}

func (x *SeckillActivity) Reset() { *x = SeckillActivity{} }
func (x *SeckillActivity) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *SeckillActivity) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *SeckillActivity) GetId() int64 { if x != nil { return x.Id }; return 0 }
func (x *SeckillActivity) GetActivityName() string { if x != nil { return x.ActivityName }; return "" }
func (x *SeckillActivity) GetProductId() int64 { if x != nil { return x.ProductId }; return 0 }
func (x *SeckillActivity) GetSkuId() int64 { if x != nil { return x.SkuId }; return 0 }
func (x *SeckillActivity) GetSeckillPrice() int64 { if x != nil { return x.SeckillPrice }; return 0 }
func (x *SeckillActivity) GetTotalStock() int32 { if x != nil { return x.TotalStock }; return 0 }
func (x *SeckillActivity) GetAvailableStock() int32 { if x != nil { return x.AvailableStock }; return 0 }
func (x *SeckillActivity) GetStartTime() int64 { if x != nil { return x.StartTime }; return 0 }
func (x *SeckillActivity) GetEndTime() int64 { if x != nil { return x.EndTime }; return 0 }
func (x *SeckillActivity) GetStatus() string { if x != nil { return x.Status }; return "" }
func (x *SeckillActivity) GetLimitPerUser() int32 { if x != nil { return x.LimitPerUser }; return 0 }

type CreateActivityReq struct {
	ActivityName string `protobuf:"bytes,1,opt,name=activity_name" json:"activity_name,omitempty"`
	ProductId    int64  `protobuf:"varint,2,opt,name=product_id" json:"product_id,omitempty"`
	SkuId        int64  `protobuf:"varint,3,opt,name=sku_id" json:"sku_id,omitempty"`
	SeckillPrice int64  `protobuf:"varint,4,opt,name=seckill_price" json:"seckill_price,omitempty"`
	TotalStock   int32  `protobuf:"varint,5,opt,name=total_stock" json:"total_stock,omitempty"`
	StartTime    int64  `protobuf:"varint,6,opt,name=start_time" json:"start_time,omitempty"`
	EndTime      int64  `protobuf:"varint,7,opt,name=end_time" json:"end_time,omitempty"`
	Status       string `protobuf:"bytes,8,opt,name=status" json:"status,omitempty"`
	LimitPerUser int32  `protobuf:"varint,9,opt,name=limit_per_user" json:"limit_per_user,omitempty"`
}

func (x *CreateActivityReq) Reset() { *x = CreateActivityReq{} }
func (x *CreateActivityReq) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *CreateActivityReq) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *CreateActivityReq) GetActivityName() string { if x != nil { return x.ActivityName }; return "" }
func (x *CreateActivityReq) GetProductId() int64 { if x != nil { return x.ProductId }; return 0 }
func (x *CreateActivityReq) GetSkuId() int64 { if x != nil { return x.SkuId }; return 0 }
func (x *CreateActivityReq) GetSeckillPrice() int64 { if x != nil { return x.SeckillPrice }; return 0 }
func (x *CreateActivityReq) GetTotalStock() int32 { if x != nil { return x.TotalStock }; return 0 }
func (x *CreateActivityReq) GetStartTime() int64 { if x != nil { return x.StartTime }; return 0 }
func (x *CreateActivityReq) GetEndTime() int64 { if x != nil { return x.EndTime }; return 0 }
func (x *CreateActivityReq) GetStatus() string { if x != nil { return x.Status }; return "" }
func (x *CreateActivityReq) GetLimitPerUser() int32 { if x != nil { return x.LimitPerUser }; return 0 }

type CreateActivityResp struct {
	ActivityId int64 `protobuf:"varint,1,opt,name=activity_id" json:"activity_id,omitempty"`
}

func (x *CreateActivityResp) Reset() { *x = CreateActivityResp{} }
func (x *CreateActivityResp) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *CreateActivityResp) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *CreateActivityResp) GetActivityId() int64 { if x != nil { return x.ActivityId }; return 0 }

type UpdateActivityStatusReq struct {
	ActivityId int64  `protobuf:"varint,1,opt,name=activity_id" json:"activity_id,omitempty"`
	Status     string `protobuf:"bytes,2,opt,name=status" json:"status,omitempty"`
}

func (x *UpdateActivityStatusReq) Reset() { *x = UpdateActivityStatusReq{} }
func (x *UpdateActivityStatusReq) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *UpdateActivityStatusReq) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *UpdateActivityStatusReq) GetActivityId() int64 { if x != nil { return x.ActivityId }; return 0 }
func (x *UpdateActivityStatusReq) GetStatus() string { if x != nil { return x.Status }; return "" }

type UpdateActivityStatusResp struct{}
func (x *UpdateActivityStatusResp) Reset() { *x = UpdateActivityStatusResp{} }
func (x *UpdateActivityStatusResp) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *UpdateActivityStatusResp) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }

type GetActivityReq struct {
	ActivityId int64 `protobuf:"varint,1,opt,name=activity_id" json:"activity_id,omitempty"`
}

func (x *GetActivityReq) Reset() { *x = GetActivityReq{} }
func (x *GetActivityReq) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *GetActivityReq) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *GetActivityReq) GetActivityId() int64 { if x != nil { return x.ActivityId }; return 0 }

type GetActivityResp struct {
	Activity *SeckillActivity `protobuf:"bytes,1,opt,name=activity" json:"activity,omitempty"`
}

func (x *GetActivityResp) Reset() { *x = GetActivityResp{} }
func (x *GetActivityResp) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *GetActivityResp) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *GetActivityResp) GetActivity() *SeckillActivity { if x != nil { return x.Activity }; return nil }

type SubmitSeckillReq struct {
	ActivityId int64 `protobuf:"varint,1,opt,name=activity_id" json:"activity_id,omitempty"`
	UserId     int64 `protobuf:"varint,2,opt,name=user_id" json:"user_id,omitempty"`
}

func (x *SubmitSeckillReq) Reset() { *x = SubmitSeckillReq{} }
func (x *SubmitSeckillReq) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *SubmitSeckillReq) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *SubmitSeckillReq) GetActivityId() int64 { if x != nil { return x.ActivityId }; return 0 }
func (x *SubmitSeckillReq) GetUserId() int64 { if x != nil { return x.UserId }; return 0 }

type SubmitSeckillResp struct {
	RequestNo string `protobuf:"bytes,1,opt,name=request_no" json:"request_no,omitempty"`
	Status    string `protobuf:"bytes,2,opt,name=status" json:"status,omitempty"`
	Message   string `protobuf:"bytes,3,opt,name=message" json:"message,omitempty"`
}

func (x *SubmitSeckillResp) Reset() { *x = SubmitSeckillResp{} }
func (x *SubmitSeckillResp) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *SubmitSeckillResp) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *SubmitSeckillResp) GetRequestNo() string { if x != nil { return x.RequestNo }; return "" }
func (x *SubmitSeckillResp) GetStatus() string { if x != nil { return x.Status }; return "" }
func (x *SubmitSeckillResp) GetMessage() string { if x != nil { return x.Message }; return "" }

type GetSeckillResultReq struct {
	RequestNo string `protobuf:"bytes,1,opt,name=request_no" json:"request_no,omitempty"`
}

func (x *GetSeckillResultReq) Reset() { *x = GetSeckillResultReq{} }
func (x *GetSeckillResultReq) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *GetSeckillResultReq) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *GetSeckillResultReq) GetRequestNo() string { if x != nil { return x.RequestNo }; return "" }

type GetSeckillResultResp struct {
	RequestNo  string `protobuf:"bytes,1,opt,name=request_no" json:"request_no,omitempty"`
	Status     string `protobuf:"bytes,2,opt,name=status" json:"status,omitempty"`
	OrderId    int64  `protobuf:"varint,3,opt,name=order_id" json:"order_id,omitempty"`
	FailReason string `protobuf:"bytes,4,opt,name=fail_reason" json:"fail_reason,omitempty"`
}

func (x *GetSeckillResultResp) Reset() { *x = GetSeckillResultResp{} }
func (x *GetSeckillResultResp) Marshal(in []byte) ([]byte, error) { return prutal.MarshalAppend(in, x) }
func (x *GetSeckillResultResp) Unmarshal(in []byte) error { return prutal.Unmarshal(in, x) }
func (x *GetSeckillResultResp) GetRequestNo() string { if x != nil { return x.RequestNo }; return "" }
func (x *GetSeckillResultResp) GetStatus() string { if x != nil { return x.Status }; return "" }
func (x *GetSeckillResultResp) GetOrderId() int64 { if x != nil { return x.OrderId }; return 0 }
func (x *GetSeckillResultResp) GetFailReason() string { if x != nil { return x.FailReason }; return "" }

type SeckillService interface {
	CreateActivity(ctx context.Context, req *CreateActivityReq) (res *CreateActivityResp, err error)
	UpdateActivityStatus(ctx context.Context, req *UpdateActivityStatusReq) (res *UpdateActivityStatusResp, err error)
	GetActivity(ctx context.Context, req *GetActivityReq) (res *GetActivityResp, err error)
	SubmitSeckill(ctx context.Context, req *SubmitSeckillReq) (res *SubmitSeckillResp, err error)
	GetSeckillResult(ctx context.Context, req *GetSeckillResultReq) (res *GetSeckillResultResp, err error)
}


