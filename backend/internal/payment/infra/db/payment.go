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
	// 瀛樺偍鏋氫妇涔熷彲浠ワ紝姣斿璇?0-CNY
	// 纾佺洏鍐呭瓨閭ｄ箞渚垮疁锛岀洿鎺ユ斁 string 涔熷彲浠?
	Currency string
	// 鍙互鎶借薄璁や负锛岃繖鏄竴涓畝鐭殑鎻忚堪
	// 涔熷氨鏄鍗充究鏄埆鐨勬敮浠樻柟寮忥紝杩欒竟涔熷彲浠ユ彁渚涗竴涓畝鍗曠殑鎻忚堪
	// 浣犲彲浠ヨ涓鸿繖绠楁槸鍐椾綑鐨勬暟鎹紝鍥犱负浠庡師鍒欎笂鏉ヨ锛屾垜浠彲浠ュ畬鍏ㄤ笉淇濆瓨鐨勩€?
	// 鑰屾槸瑕佹眰璋冪敤鑰呯洿鎺?BizID 鍜?Biz 鍘绘壘涓氬姟鏂硅
	// 绠″緱瓒婂皯锛岀郴缁熻秺绋?
	Description string `gorm:"description"`
	// 鍚庣画鍙互鑰冭檻澧炲姞瀛楁锛屾潵鏍囪鏄敤鐨勬槸寰俊鏀粯鎴栨敮浠樺疂鏀粯
	// 涔熷彲浠ヨ€冭檻鎻愪緵涓€涓法澶х殑 BLOB 瀛楁锛?
	// 鏉ュ瓨鍌ㄥ拰鏀粯鏈夊叧鐨勫叾瀹冨瓧娈?
	// ExtraData
	// 涓氬姟鏂逛紶杩囨潵鐨勶紝涓氬姟鏍囪瘑
	BizTradeNO string `gorm:"column:biz_trade_no;type:varchar(256);unique;index"`

	// 绗笁鏂规敮浠樺钩鍙扮殑浜嬪姟 ID锛屽敮涓€鐨勶紝涓斿彲鑳戒负绌猴紝鐢╯ql.NullString
	TxnID sql.NullString `gorm:"column:txn_id;type:varchar(128);unique"`
}


