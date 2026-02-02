package db

import (
	"database/sql"
	"time"
)

type Payment struct {
	ID        int64 `gorm:"primary_key,AUTO_INCREMENT"`
	CreatedAt time.Time
	Status    uint8     `gorm:"column:status;default:1;index:status_updateAt"`
	UpdatedAt time.Time `gorm:"index:status_updateAt"`

	Amt int64
	// 存储枚举也可以，比如说 0-CNY
	// 磁盘内存那么便宜，直接放 string 也可以
	Currency string
	// 可以抽象认为，这是一个简短的描述
	// 也就是说即便是别的支付方式，这边也可以提供一个简单的描述
	// 你可以认为这算是冗余的数据，因为从原则上来说，我们可以完全不保存的。
	// 而是要求调用者直接 BizID 和 Biz 去找业务方要
	// 管得越少，系统越稳
	Description string `gorm:"description"`
	// 后续可以考虑增加字段，来标记是用的是微信支付或支付宝支付
	// 也可以考虑提供一个巨大的 BLOB 字段，
	// 来存储和支付有关的其它字段
	// ExtraData
	// 业务方传过来的，业务标识
	BizTradeNO string `gorm:"column:biz_trade_no;type:varchar(256);unique;index"`

	// 第三方支付平台的事务 ID，唯一的，且可能为空，用sql.NullString
	TxnID sql.NullString `gorm:"column:txn_id;type:varchar(128);unique"`
}
