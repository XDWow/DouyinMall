package db

import "time"

// Inventory 库存表
type Inventory struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	ProductID int64 `gorm:"uniqueIndex;not null"`
	Stock     int64 `gorm:"not null"` // 当前可用库存
	Sold      int64 `gorm:"not null;default:0"` // 已售出数量（累计，用于审计和数据分析）
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InventoryOperation 库存操作记录表（商品级幂等 + 恢复）一定要记得有两个作用，老是忘
type InventoryOperation struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	OperationID string    `gorm:"uniqueIndex:idx_op_prod;size:128;not null"` // 业务幂等键（如：order:123:reserve）
	ProductID   int64     `gorm:"uniqueIndex:idx_op_prod;index:idx_product;not null"`
	Type        string    `gorm:"index;size:32;not null"` // 操作类型：commit/refund/adjust（方便统计）
	Reason      string    `gorm:"size:255"`               // 原因（adjust 时需要，方便审计）
	Quantity    int32     `gorm:"not null"`               // 变动数量（正数=增加，负数=减少）
	CreatedAt   time.Time `gorm:"index"`

	// 联合唯一索引：(operation_id, product_id) 保证商品级幂等，支持部分退款
}

func (Inventory) TableName() string {
	return "inventory"
}

func (InventoryOperation) TableName() string {
	return "inventory_operation"
}

/*
架构演进记录：

v2 - 为什么不需要主表 InventoryOperation：
主表原本用于关联多个商品的操作，但实际上：
- 幂等已经在 item 表通过 (operation_id, product_id) 联合唯一索引实现
- 查询操作的所有商品：直接 WHERE operation_id = 'xxx' 查 item 表即可
- Type 和 Reason 虽然会重复存储（同一操作多个商品），但简化了架构，查询更直接
- 如果未来需要操作级别的额外信息（操作人、IP等），再加主表也不迟

// type InventoryOperation struct {
// 	ID          int64  `gorm:"primaryKey;autoIncrement"`
// 	OperationID string `gorm:"uniqueIndex;size:128;not null"`
// 	Type        string `gorm:"size:32;not null"`
// 	Reason      string `gorm:"size:255"`
// 	CreatedAt   time.Time
// }

v1 - 失败的设计（数组字段）：
// type InventoryOperation struct {
// 	ID          int64   `gorm:"primaryKey;autoIncrement"`
// 	OperationID string  `gorm:"uniqueIndex"`
// 	ProductID   []int64
// 	Quantity    []int32
// 	CreatedAt   time.Time
// }

为什么失败：
1、部分退款下的幂等 & 一致性无法保证，必须按商品维度拆分以支持商品级幂等、重试和补偿
2、审计，查库存为什么不对，只能全表扫描，ProductID 走不了索引
*/
