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
		// 浣犺繖閲屽笇鏈涘畬鎴愬弻鍐?
		// 杩欓噷鍙湁涓€涓?db 杩囨潵锛屼綘瑕佷箞鏄?src锛岃涔堟槸 dst
		// 鍋氫笉鍒板姩鎬佸垏鎹?
		// 杩欓噷浣犳敼涓嶄簡鐨?
		// d.src.Create(db.Statement.Model).Error
	}
}


