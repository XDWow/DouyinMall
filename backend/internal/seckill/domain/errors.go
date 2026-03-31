package domain

import "errors"

var (
	ErrActivityNotFound   = errors.New("activity not found")
	ErrActivityNotStarted = errors.New("activity not started")
	ErrActivityEnded      = errors.New("activity ended")
	ErrActivityOffline    = errors.New("activity offline")
	ErrDuplicateSeckill   = errors.New("duplicate seckill")
	ErrOutOfStock         = errors.New("out of stock")
	ErrRequestNotFound    = errors.New("request not found")
	ErrInvalidStatus      = errors.New("invalid status")
)

