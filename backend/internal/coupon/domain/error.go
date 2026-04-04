package domain

import "errors"

var (
	ErrCouponNotFound         = errors.New("浼樻儬鍒镐笉瀛樺湪")
	ErrCouponNotAvailable     = errors.New("浼樻儬鍒镐笉鍙敤")
	ErrCouponExpired          = errors.New("浼樻儬鍒稿凡杩囨湡")
	ErrCouponAlreadyUsed      = errors.New("浼樻儬鍒稿凡浣跨敤")
	ErrCouponAlreadyLocked    = errors.New("浼樻儬鍒稿凡琚叾浠栬鍗曢攣瀹?)
	ErrCouponNotLocked        = errors.New("浼樻儬鍒告湭閿佸畾")
	ErrCouponNotOwned         = errors.New("浼樻儬鍒镐笉灞炰簬褰撳墠鐢ㄦ埛")
	ErrThresholdNotMet        = errors.New("鏈揪鍒颁娇鐢ㄩ棬妲?)
	ErrOrderNotApplicable     = errors.New("璁㈠崟涓棤閫傜敤鍟嗗搧")
	ErrDuplicateOperation     = errors.New("閲嶅鎿嶄綔")
	ErrOperationNotFound      = errors.New("鎿嶄綔璁板綍涓嶅瓨鍦?)
	ErrIssueLimitExceeded     = errors.New("宸茶揪棰嗗彇涓婇檺")
	ErrCouponLimitExceeded    = errors.New("宸茶揪棰嗗彇涓婇檺")
	ErrTemplateStockOut       = errors.New("浼樻儬鍒稿凡鍙戝畬")
	ErrCouponTemplateNotFound = errors.New("浼樻儬鍒告ā鏉夸笉瀛樺湪")
	ErrCouponCannotIssue      = errors.New("浼樻儬鍒镐笉鍙彂鏀?)
)


