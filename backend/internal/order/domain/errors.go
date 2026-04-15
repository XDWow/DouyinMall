package domain

import "errors"

// ErrRecordNotFound 表示记录不存在。
var ErrRecordNotFound = errors.New("record not found")
