package domain

import (
	"errors"
	"fmt"
)

var ErrDuplicateOperation = errors.New("鎿嶄綔宸叉墽琛岃繃")

var ErrProductNotFound = errors.New("鍟嗗搧涓嶅瓨鍦?)

var ErrInsufficientStock = errors.New("搴撳瓨涓嶈冻") // CommitStock DB 灞傜敤

// InsufficientStockItem 鍗曚釜鍟嗗搧鐨勫簱瀛樹笉瓒虫槑缁?
type InsufficientStockItem struct {
	ProductID int64
	Requested int64 // 璇锋眰棰勬墸鏁伴噺
	Available int64 // 鍙敤搴撳瓨 = 瀹為檯搴撳瓨 - Redis 宸查鎵ｅ簱瀛?
}

// InsufficientStockError 棰勬墸搴撳瓨澶辫触锛屽甫鏈夋墍鏈変笉瓒冲晢鍝佺殑鏄庣粏
type InsufficientStockError struct {
	Items []InsufficientStockItem
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("%d 浠跺晢鍝佸簱瀛樹笉瓒?, len(e.Items))
}


