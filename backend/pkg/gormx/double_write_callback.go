package gormx

import (
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"gorm.io/gorm"
)

type DoubleWriteCallback struct {
	src     *gorm.DB
	dst     *gorm.DB
	pattern *atomicx.Value[string]
}

func (d *DoubleWriteCallback) create() func(db *gorm.DB) {
	return func(db *gorm.DB) {
		// 你这里希望完成双写
		// 这里只有一个 db 过来，你要么是 src，要么是 dst
		// 做不到动态切换
		// 这里你改不了的
		// d.src.Create(db.Statement.Model).Error
	}
}
