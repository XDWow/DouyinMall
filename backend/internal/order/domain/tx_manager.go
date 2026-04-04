package domain

import "context"

// TxManager 浜嬪姟绠＄悊鍣ㄦ帴鍙ｏ紝鐢ㄤ簬澶勭悊鏁版嵁搴撲簨鍔?
// 鏀惧湪domain灞傝€屼笉鏄痷secase灞傦紝閬垮厤寰幆渚濊禆
type TxManager interface {
	// Tx 鍦ㄤ簨鍔′腑鎵ц鎿嶄綔
	// 濡傛灉fn杩斿洖error锛屼簨鍔′細鍥炴粴锛涘惁鍒欐彁浜?
	Tx(ctx context.Context, fn func(context.Context) error) error
}


