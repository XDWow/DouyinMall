package domain

import "errors"

var (
	ErrActivityNotFound   = errors.New("活动不存在")
	ErrActivityNotStarted = errors.New("活动未开始")
	ErrActivityEnded      = errors.New("活动已结束")
	ErrActivityOffline    = errors.New("活动未上线或已下线")
	ErrDuplicateSeckill   = errors.New("重复秒杀请求")
	ErrOutOfStock         = errors.New("库存不足")
	ErrRequestNotFound    = errors.New("请求记录不存在")
	ErrInvalidStatus      = errors.New("状态非法")
)
