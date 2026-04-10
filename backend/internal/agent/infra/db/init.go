package db

import "gorm.io/gorm"

// InitTables 创建/更新 agent 相关表结构（与 coupon/infra/db 一致，启动时调用）。
func InitTables(database *gorm.DB) error {
	return database.AutoMigrate(
		&Session{},
		&Message{},
		&KnowledgeChunk{},
	)
}
