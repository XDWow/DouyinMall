package db

import "time"

// Inventory 搴撳瓨琛?
type Inventory struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	ProductID int64 `gorm:"uniqueIndex;not null"`
	Stock     int64 `gorm:"not null"` // 褰撳墠鍙敤搴撳瓨
	Sold      int64 `gorm:"not null;default:0"` // 宸插敭鍑烘暟閲忥紙绱锛岀敤浜庡璁″拰鏁版嵁鍒嗘瀽锛?
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InventoryOperation 搴撳瓨鎿嶄綔璁板綍琛紙鍟嗗搧绾у箓绛?+ 鎭㈠锛変竴瀹氳璁板緱鏈変袱涓綔鐢紝鑰佹槸蹇?
type InventoryOperation struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	OperationID string    `gorm:"uniqueIndex:idx_op_prod;size:128;not null"` // 涓氬姟骞傜瓑閿紙濡傦細order:123:reserve锛?
	ProductID   int64     `gorm:"uniqueIndex:idx_op_prod;index:idx_product;not null"`
	Type        string    `gorm:"index;size:32;not null"` // 鎿嶄綔绫诲瀷锛歝ommit/refund/adjust锛堟柟渚跨粺璁★級
	Reason      string    `gorm:"size:255"`               // 鍘熷洜锛坅djust 鏃堕渶瑕侊紝鏂逛究瀹¤锛?
	Quantity    int32     `gorm:"not null"`               // 鍙樺姩鏁伴噺锛堟鏁?澧炲姞锛岃礋鏁?鍑忓皯锛?
	CreatedAt   time.Time `gorm:"index"`

	// 鑱斿悎鍞竴绱㈠紩锛?operation_id, product_id) 淇濊瘉鍟嗗搧绾у箓绛夛紝鏀寔閮ㄥ垎閫€娆?
}

func (Inventory) TableName() string {
	return "inventory"
}

func (InventoryOperation) TableName() string {
	return "inventory_operation"
}

/*
鏋舵瀯婕旇繘璁板綍锛?

v2 - 涓轰粈涔堜笉闇€瑕佷富琛?InventoryOperation锛?
涓昏〃鍘熸湰鐢ㄤ簬鍏宠仈澶氫釜鍟嗗搧鐨勬搷浣滐紝浣嗗疄闄呬笂锛?
- 骞傜瓑宸茬粡鍦?item 琛ㄩ€氳繃 (operation_id, product_id) 鑱斿悎鍞竴绱㈠紩瀹炵幇
- 鏌ヨ鎿嶄綔鐨勬墍鏈夊晢鍝侊細鐩存帴 WHERE operation_id = 'xxx' 鏌?item 琛ㄥ嵆鍙?
- Type 鍜?Reason 铏界劧浼氶噸澶嶅瓨鍌紙鍚屼竴鎿嶄綔澶氫釜鍟嗗搧锛夛紝浣嗙畝鍖栦簡鏋舵瀯锛屾煡璇㈡洿鐩存帴
- 濡傛灉鏈潵闇€瑕佹搷浣滅骇鍒殑棰濆淇℃伅锛堟搷浣滀汉銆両P绛夛級锛屽啀鍔犱富琛ㄤ篃涓嶈繜

// type InventoryOperation struct {
// 	ID          int64  `gorm:"primaryKey;autoIncrement"`
// 	OperationID string `gorm:"uniqueIndex;size:128;not null"`
// 	Type        string `gorm:"size:32;not null"`
// 	Reason      string `gorm:"size:255"`
// 	CreatedAt   time.Time
// }

v1 - 澶辫触鐨勮璁★紙鏁扮粍瀛楁锛夛細
// type InventoryOperation struct {
// 	ID          int64   `gorm:"primaryKey;autoIncrement"`
// 	OperationID string  `gorm:"uniqueIndex"`
// 	ProductID   []int64
// 	Quantity    []int32
// 	CreatedAt   time.Time
// }

涓轰粈涔堝け璐ワ細
1銆侀儴鍒嗛€€娆句笅鐨勫箓绛?& 涓€鑷存€ф棤娉曚繚璇侊紝蹇呴』鎸夊晢鍝佺淮搴︽媶鍒嗕互鏀寔鍟嗗搧绾у箓绛夈€侀噸璇曞拰琛ュ伩
2銆佸璁★紝鏌ュ簱瀛樹负浠€涔堜笉瀵癸紝鍙兘鍏ㄨ〃鎵弿锛孭roductID 璧颁笉浜嗙储寮?
*/


