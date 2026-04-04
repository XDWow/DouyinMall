//go:build legacy_agent

package db

import "gorm.io/gorm"

func InitTables(db *gorm.DB) error {
	return db.AutoMigrate(&Session{}, &Message{}, &KnowledgeItem{})
}


